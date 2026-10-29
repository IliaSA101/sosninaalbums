package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Пределы вызова инструмента (design §3).
const (
	maxReply    = 1<<20 - 2048     // строка ответа на проводе, байт: запас под конверт JSON-RPC
	maxCalls    = 4                // одновременных вызовов на службу
	callTimeout = 10 * time.Second // срок одного вызова
)

// toolSpec — строка таблицы инструментов (tools.go).
type toolSpec struct {
	name     string
	title    string // название для журнала и клиента
	desc     string
	minLevel int  // уровень доступа, с которого инструмент отвечает (3 — картинки)
	paged    bool // у инструмента есть limit: слишком большой ответ лечится им
	slow     bool // рендер: срок вдвое длиннее (design §6: 20 с вместо 10)

	// Ступень 3 (149, edit.go).
	edit   bool // правит альбом: путь serveEdit, аннотации «не только чтение», ✎ в журнале
	view   bool // меняет вид окна, а не альбом (show_spread): место, темп и «человек занят» — без флажка и копии
	await  bool // продолжение вопроса (await_answer): ни семафора чтения, ни места правки
	ask    bool // вопрос человеку: срок вызова AskWait + CallTimeout
	auto   bool // автосборка: срок вызова AutoTimeout
	backup bool // до тела — резервная копия (Host.Backup)
	danger bool // DestructiveHint: заменяет открытый альбом
}

// toolEnv — общее для инструментов одного запуска службы: сервер, место под вызовы, сроки.
type toolEnv struct {
	s   *Service
	r   *run
	srv *mcp.Server
	ctx context.Context // жизнь запуска службы: вопросы агента живут дольше вызова
	sem chan struct{}   // занятые места вызовов
	// render — место под один рендер разворота: они тяжёлые и идут по одному (design §6).
	render chan struct{}
}

func newToolEnv(s *Service, r *run) *toolEnv {
	n := s.o.MaxCalls
	return &toolEnv{s: s, r: r, srv: r.srv, ctx: r.ctx, sem: make(chan struct{}, n), render: make(chan struct{}, 1)}
}

// call — что видит тело инструмента: контекст с сроком, уровень доступа на этот вызов и служба.
type call struct {
	ctx   context.Context
	s     *Service
	env   *toolEnv
	level int
	spec  toolSpec
	sess  *mcp.ServerSession   // подключение вызова; запись отмены и жетон вопроса привязаны к нему
	rel   func()               // место правки (edit.go); берёт тот, кто его занял
	keep  bool                 // место правки передано вопросу: отпустит он
	cats  map[string]*decorCat // каталоги оформления, прочитанные в этом вызове
}

// reg регистрирует инструмент низкоуровневым AddTool: типизированный mcp.AddTool отдавал бы агенту
// текст любой ошибки как есть (в нём бывают пути), здесь наружу идут только agentErr.
func reg[In, Out any](e *toolEnv, spec toolSpec, tweak func(props map[string]*jsonschema.Schema), run func(*call, In) (Out, error)) {
	in, err := jsonschema.For[In](nil)
	if err != nil {
		panic("agents: схема входа " + spec.name + ": " + err.Error())
	}
	out, err := jsonschema.For[Out](nil)
	if err != nil {
		panic("agents: схема выхода " + spec.name + ": " + err.Error())
	}
	if tweak != nil {
		tweak(in.Properties)
	}
	resolved, err := in.Resolve(nil)
	if err != nil {
		panic("agents: схема входа " + spec.name + ": " + err.Error())
	}
	e.srv.AddTool(&mcp.Tool{
		Name: spec.name, Title: spec.title, Description: spec.desc,
		InputSchema: in, OutputSchema: out, Annotations: spec.annotations(),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (res *mcp.CallToolResult, _ error) {
		start := time.Now()
		var note string
		defer func() { e.s.recordNote(spec, req, res, time.Since(start), note) }() // после recover ниже: паника тоже вызов
		defer func() {                                                             // SDK паник не ловит, а горутина обработчика без recover роняет программу
			if p := recover(); p != nil {
				logPanic("tool "+spec.name, p)
				res = errResult(errInternal)
			}
		}()
		var raw json.RawMessage
		if req != nil && req.Params != nil {
			raw = req.Params.Arguments
		}
		var sess *mcp.ServerSession
		if req != nil {
			sess = req.Session
		}
		return e.serve(ctx, spec, in, resolved, raw, serveOpts{sess: sess, note: &note}, func(c *call, raw json.RawMessage) (any, error) {
			var a In
			if len(raw) > 0 {
				d := json.NewDecoder(bytes.NewReader(raw))
				d.DisallowUnknownFields()
				if err := d.Decode(&a); err != nil {
					return nil, badArgs(textArgsUnknown)
				}
			}
			return run(c, a)
		})
	})
}

// serveOpts — что вызов приносит с собой: подключение и место для итога журнала (Call.Note).
type serveOpts struct {
	sess *mcp.ServerSession
	note *string
}

// annotations — подсказки клиенту (не защита: MCP 2025-11-25 считает их недоверенными). Чтение —
// «только чтение»; правка — явно «не только чтение, не идемпотентна, закрытый мир», разрушающей
// помечена лишь замена открытого альбома (0.33-149-5).
func (spec toolSpec) annotations() *mcp.ToolAnnotations {
	f, t := false, true
	if !spec.edit && !spec.view && !spec.await {
		return &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &f}
	}
	d := f
	if spec.danger {
		d = t
	}
	return &mcp.ToolAnnotations{ReadOnlyHint: false, IdempotentHint: spec.view, DestructiveHint: &d, OpenWorldHint: &f}
}

// serve — общий путь вызова: вход по схеме, уровень, место, срок, ответ и его предел.
func (e *toolEnv) serve(ctx context.Context, spec toolSpec, schema *jsonschema.Schema, rs *jsonschema.Resolved, raw json.RawMessage, so serveOpts,
	exec func(*call, json.RawMessage) (any, error)) (*mcp.CallToolResult, error) {
	var v any = map[string]any{}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &v); err != nil {
			return errResult(badArgs(textArgsNotJSON)), nil
		}
		if v == nil { // "arguments": null — как без аргументов
			v = map[string]any{}
		}
	}
	if err := rs.Validate(&v); err != nil {
		why := explainArgs(schema, v)
		if why == "" { // нарушение, которого разбор не знает: общая фраза, текст SDK наружу не идёт
			why = textArgsUnknown
		}
		return errResult(badArgs(why)), nil
	}
	// Место — до чтения уровня: поток вызовов при занятых местах не дёргает config.json.
	// await_answer мест чтения не занимает: ждёт чужого ответа, а читать не мешает (В12).
	if !spec.await {
		select {
		case e.sem <- struct{}{}:
		default:
			return errResult(errCallBusy), nil
		}
	}
	lv := e.s.level()
	if lv < spec.minLevel {
		if !spec.await {
			<-e.sem
		}
		return errResult(errNoImages), nil
	}
	val, err := e.invoke(ctx, &call{s: e.s, env: e, level: lv, spec: spec, sess: so.sess}, spec, raw, exec)
	if err != nil {
		ae, orig := toAgent(err)
		if so.note != nil {
			*so.note = noteOf(spec, ae)
		}
		switch {
		case orig != nil:
			log.Printf("agents: %s: %v", spec.name, bare(orig))
		case ae == errTimeout: // зависшее тело держит место: в log.txt видно, какое (без данных)
			log.Printf("agents: %s не уложился в срок", spec.name)
		}
		return errResult(ae), nil
	}
	return okResult(val, spec.paged), nil
}

// invoke (место уже занято в serve) исполняет тело в своей горутине: место освобождается по её концу, а не по сроку — агент
// получает «время вышло», но пятый вызов не стартует, пока висит четвёртый (design §2).
func (e *toolEnv) invoke(ctx context.Context, c *call, spec toolSpec, raw json.RawMessage, exec func(*call, json.RawMessage) (any, error)) (any, error) {
	cctx, cancel := context.WithTimeout(ctx, e.s.budget(spec))
	c.ctx = cctx
	type result struct {
		v   any
		err error
	}
	ch := make(chan result, 1)
	go func() {
		defer func() {
			if !spec.await {
				<-e.sem
			}
		}()
		defer cancel()
		defer func() {
			if p := recover(); p != nil {
				logPanic("tool body", p)
				ch <- result{err: errInternal}
			}
		}()
		v, err := exec(c, raw)
		ch <- result{v, err}
	}()
	select {
	case r := <-ch:
		return r.v, r.err
	case <-cctx.Done():
		select { // тело успело закончить: cancel горутины и ответ готовы одновременно
		case r := <-ch:
			return r.v, r.err
		default:
		}
		if errors.Is(cctx.Err(), context.DeadlineExceeded) || ctx.Err() != nil {
			return nil, errTimeout
		}
		return nil, errInternal
	}
}

func errResult(err *agentErr) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}
}

// okResult — ответ: тот же JSON и текстом (Content), и структурой (для старых и новых клиентов).
// Размер — строка ответа на проводе; больше предела — ошибка из списка, а не обрезанные данные.
func okResult(v any, paged bool) *mcp.CallToolResult {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("agents: ответ не собран (%T)", err)
		return errResult(errInternal)
	}
	res := &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(b)}},
		StructuredContent: json.RawMessage(b),
	}
	if im, ok := v.(withImage); ok { // превью: подпись текстом, затем сама картинка
		res.Content = append(res.Content, &mcp.ImageContent{Data: im.jpegBytes(), MIMEType: "image/jpeg"})
	}
	if w, err := json.Marshal(res); err != nil || len(w) > maxReply {
		if paged {
			return errResult(errTooBig)
		}
		return errResult(errTooBigOne)
	}
	return res
}

// budget — срок одного вызова: рендер вдвое длиннее, вопрос — плюс ожидание ответа, автосборка — своя.
func (s *Service) budget(spec toolSpec) time.Duration {
	switch {
	case spec.slow:
		return s.o.CallTimeout * 2
	case spec.ask:
		return s.o.AskWait + s.o.CallTimeout
	case spec.auto:
		return s.o.AutoTimeout
	}
	return s.o.CallTimeout
}

// level — уровень доступа на этот вызов (смена в окне действует сразу).
func (s *Service) level() int { return levelOf(s.h.CfgDir()) }
