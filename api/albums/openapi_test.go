package albums

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

// schemaTypes — какой тип Go описывает каждая схема components.schemas.
var schemaTypes = map[string]reflect.Type{
	"Manifest": reflect.TypeFor[Manifest](), "Book": reflect.TypeFor[Book](), "Size": reflect.TypeFor[Size](),
	"Page": reflect.TypeFor[Page](), "Limits": reflect.TypeFor[Limits](), "Health": reflect.TypeFor[Health](),
	"CreateRequest": reflect.TypeFor[CreateRequest](), "UpdateRequest": reflect.TypeFor[UpdateRequest](),
	"Patch": reflect.TypeFor[Patch](), "Created": reflect.TypeFor[Created](), "Updated": reflect.TypeFor[Updated](),
	"Published": reflect.TypeFor[Published](), "Album": reflect.TypeFor[Album](), "AlbumList": reflect.TypeFor[AlbumList](),
	"Error": reflect.TypeFor[Error](), "ErrorBody": reflect.TypeFor[ErrorBody](),
}

// operations — operationId → «МЕТОД путь» и метод клиента с тем же именем.
var operations = map[string]string{
	"Health": "GET /v1/health", "List": "GET /v1/albums", "Create": "POST /v1/albums",
	"Update": "PUT /v1/albums/{id}", "Patch": "PATCH /v1/albums/{id}", "Delete": "DELETE /v1/albums/{id}",
	"PutPage": "PUT /v1/albums/{id}/pages/{n}", "Publish": "POST /v1/albums/{id}/publish",
}

type m = map[string]any

func loadDoc(t *testing.T) m {
	t.Helper()
	var doc m
	if err := yaml.Unmarshal(OpenAPI, &doc); err != nil {
		t.Fatalf("openapi.yaml не разбирается: %v", err)
	}
	return doc
}

func sub(t *testing.T, v any, keys ...string) m {
	t.Helper()
	cur, ok := v.(m)
	for _, k := range keys {
		if !ok {
			t.Fatalf("нет раздела %v", keys)
		}
		cur, ok = cur[k].(m)
	}
	if !ok {
		t.Fatalf("нет раздела %v", keys)
	}
	return cur
}

func num(v any) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case float64:
		return x
	}
	return -1
}

// jsonFields — теги json полей структуры: имя → omitempty.
func jsonFields(typ reflect.Type) map[string]bool {
	out := map[string]bool{}
	for f := range typ.Fields() {
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		out[name] = strings.Contains(opts, "omitempty")
	}
	return out
}

// TestOpenAPISync: каждое поле схемы ↔ тег json типа, каждый путь и метод ↔ запрос клиента.
func TestOpenAPISync(t *testing.T) {
	doc := loadDoc(t)
	schemas := sub(t, doc, "components", "schemas")
	var names, want []string
	for n := range schemas {
		names = append(names, n)
	}
	for n := range schemaTypes {
		want = append(want, n)
	}
	sort.Strings(names)
	sort.Strings(want)
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("схемы %v, типы %v", names, want)
	}
	for name, typ := range schemaTypes {
		sch := sub(t, schemas, name)
		props := sub(t, sch, "properties")
		fields := jsonFields(typ)
		for p := range props {
			if _, ok := fields[p]; !ok {
				t.Errorf("%s.%s есть в OpenAPI, в типе нет", name, p)
			}
		}
		required := map[string]bool{}
		if r, ok := sch["required"].([]any); ok {
			for _, x := range r {
				required[x.(string)] = true
			}
		}
		for f, omit := range fields {
			if _, ok := props[f]; !ok {
				t.Errorf("%s.%s есть в типе, в OpenAPI нет", name, f)
			}
			if required[f] == omit {
				t.Errorf("%s.%s: required=%v при omitempty=%v", name, f, required[f], omit)
			}
		}
		for f := range typ.Fields() {
			tag, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if p, ok := props[tag].(m); ok {
				checkKind(t, name+"."+tag, p, f.Type)
			}
		}
	}

	checkLimits(t, doc, schemas)
}

// checkLimits: пределы формата в схемах совпадают с константами, коды ошибок — с Code*.
func checkLimits(t *testing.T, doc, schemas m) {
	t.Helper()
	lim := func(path ...string) float64 {
		return num(sub(t, schemas, path[:len(path)-1]...)[path[len(path)-1]])
	}
	for what, c := range map[string][2]float64{
		"Manifest.title.maxLength":       {lim("Manifest", "properties", "title", "maxLength"), MaxTitle},
		"Manifest.generator.maxLength":   {lim("Manifest", "properties", "generator", "maxLength"), MaxGenerator},
		"CreateRequest.password.max":     {lim("CreateRequest", "properties", "password", "maxLength"), MaxPassword},
		"Patch.password.max":             {lim("Patch", "properties", "password", "maxLength"), MaxPassword},
		"Size.w.min":                     {lim("Size", "properties", "w", "minimum"), MinPageMM},
		"Size.w.max":                     {lim("Size", "properties", "w", "maximum"), MaxPageMM},
		"Size.h.min":                     {lim("Size", "properties", "h", "minimum"), MinPageMM},
		"Size.h.max":                     {lim("Size", "properties", "h", "maximum"), MaxPageMM},
		"Page.w.max":                     {lim("Page", "properties", "w", "maximum"), MaxSide},
		"Page.h.max":                     {lim("Page", "properties", "h", "maximum"), MaxSide},
		"Page.sha256.maxLength":          {lim("Page", "properties", "sha256", "maxLength"), 64},
		"Created.id.minLength":           {lim("Created", "properties", "id", "minLength"), MinID},
		"Created.id.maxLength":           {lim("Created", "properties", "id", "maxLength"), MaxID},
		"Album.id.minLength":             {lim("Album", "properties", "id", "minLength"), MinID},
		"Album.id.maxLength":             {lim("Album", "properties", "id", "maxLength"), MaxID},
		"Created.url.maxLength":          {lim("Created", "properties", "url", "maxLength"), MaxURL},
		"Album.url.maxLength":            {lim("Album", "properties", "url", "maxLength"), MaxURL},
		"Album.title.maxLength":          {lim("Album", "properties", "title", "maxLength"), MaxTitle},
		"Limits.title_len.maximum":       {lim("Limits", "properties", "title_len", "maximum"), MaxTitle},
		"Page.file.maxLength (на входе)": {lim("Page", "properties", "file", "maxLength"), 0},
	} {
		if c[0] != c[1] {
			t.Errorf("%s: в OpenAPI %v, в коде %v", what, c[0], c[1])
		}
	}
	idParam := sub(t, doc, "components", "parameters", "id", "schema")
	if num(idParam["minLength"]) != MinID || num(idParam["maxLength"]) != MaxID {
		t.Errorf("параметр id: %v", idParam)
	}

	// коды ошибок
	var codes []string
	for _, c := range sub(t, schemas, "Error", "properties", "code")["enum"].([]any) {
		codes = append(codes, c.(string))
	}
	wantCodes := []string{CodeBadManifest, CodeBadPage, CodeUnauthorized, CodeNotFound, CodeHashMismatch, CodePagesMissing, CodeTooLarge, CodeRateLimited, CodeInternal}
	sort.Strings(codes)
	sort.Strings(wantCodes)
	if !reflect.DeepEqual(codes, wantCodes) {
		t.Errorf("коды ошибок: OpenAPI %v, код %v", codes, wantCodes)
	}
}

// checkKind сверяет тип схемы с типом поля Go.
func checkKind(t *testing.T, what string, prop m, typ reflect.Type) {
	t.Helper()
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if ref, ok := prop["$ref"].(string); ok {
		name := ref[strings.LastIndex(ref, "/")+1:]
		if schemaTypes[name] != typ {
			t.Errorf("%s: $ref %s, тип Go %v", what, name, typ)
		}
		return
	}
	got, _ := prop["type"].(string)
	var want string
	switch {
	case typ == reflect.TypeFor[time.Time]():
		want = "string"
		if prop["format"] != "date-time" {
			t.Errorf("%s: время без format date-time", what)
		}
	case typ.Kind() == reflect.String:
		want = "string"
	case typ.Kind() == reflect.Bool:
		want = "boolean"
	case typ.Kind() >= reflect.Int && typ.Kind() <= reflect.Int64:
		want = "integer"
	case typ.Kind() == reflect.Float64:
		want = "number"
	case typ.Kind() == reflect.Slice:
		want = "array"
		if items, ok := prop["items"].(m); ok {
			checkKind(t, what+"[]", items, typ.Elem())
		}
	}
	if got != want {
		t.Errorf("%s: тип схемы %q, тип Go %v (%q)", what, got, typ, want)
	}
}

// TestOpenAPIPaths: пути и методы документа — ровно те, что шлёт клиент; operationId — имя метода Client.
func TestOpenAPIPaths(t *testing.T) {
	doc := loadDoc(t)
	got := map[string]string{} // operationId → «МЕТОД путь»
	for path, item := range sub(t, doc, "paths") {
		for method, op := range item.(m) {
			if method == "parameters" {
				continue
			}
			id, _ := op.(m)["operationId"].(string)
			got[id] = strings.ToUpper(method) + " " + path
		}
	}
	if !reflect.DeepEqual(got, operations) {
		t.Errorf("операции OpenAPI %v, ожидалось %v", got, operations)
	}
	// у каждой операции есть метод клиента; лишних методов API у клиента нет
	ct := reflect.TypeFor[*Client]()
	for id := range operations {
		if _, ok := ct.MethodByName(id); !ok {
			t.Errorf("у Client нет метода %s", id)
		}
	}
	for meth := range ct.Methods() {
		if _, ok := operations[meth.Name]; !ok && meth.Name != "Base" && meth.Name != "Limits" {
			t.Errorf("метод Client %s не описан в OpenAPI", meth.Name)
		}
	}
	// клиент действительно шлёт «МЕТОД путь» из документа
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		w.WriteHeader(500)
	}))
	defer srv.Close()
	c, err := New(srv.URL, testToken, Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	exp := time.Now().Add(time.Hour)
	pw := ""
	_, _ = c.Health(ctx)
	_, _ = c.List(ctx)
	_, _ = c.Create(ctx, &CreateRequest{Manifest: *goodManifest(4, false), ExpiresAt: exp})
	_, _ = c.Update(ctx, testID, goodManifest(4, false))
	_, _ = c.Patch(ctx, testID, &Patch{Password: &pw})
	_ = c.Delete(ctx, testID)
	_ = c.PutPage(ctx, testID, 3, TypeJPEG, strings.NewReader("x"), 1)
	_, _ = c.Publish(ctx, testID)
	var wantSeen []string
	for _, id := range []string{"Health", "List", "Create", "Update", "Patch", "Delete", "PutPage", "Publish"} {
		p := strings.ReplaceAll(strings.ReplaceAll(operations[id], "{id}", testID), "{n}", strconv.Itoa(3))
		wantSeen = append(wantSeen, p)
	}
	if !reflect.DeepEqual(seen, wantSeen) {
		t.Errorf("клиент шлёт\n%v\nдокумент\n%v", seen, wantSeen)
	}
}
