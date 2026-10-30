package albums

import (
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// strEnum — enum схемы строкой.
func strEnum(t *testing.T, prop m) []string {
	t.Helper()
	var out []string
	list, _ := prop["enum"].([]any)
	for _, v := range list {
		out = append(out, v.(string))
	}
	sort.Strings(out)
	return out
}

// TestOpenAPIEnums: значения role и type в схеме Page — ровно константы пакета.
func TestOpenAPIEnums(t *testing.T) {
	doc := loadDoc(t)
	page := sub(t, doc, "components", "schemas", "Page", "properties")
	for what, c := range map[string][2][]string{
		"Page.role": {strEnum(t, sub(t, page, "role")), {RoleBack, RoleFront, RolePage}},
		"Page.type": {strEnum(t, sub(t, page, "type")), {TypeJPEG, TypeWebP}},
	} {
		if !reflect.DeepEqual(c[0], c[1]) {
			t.Errorf("%s: в OpenAPI %v, в коде %v", what, c[0], c[1])
		}
	}
}

// TestOpenAPIPatterns: pattern id и sha256 в схемах принимают и отвергают то же, что CheckID и isSHA256.
func TestOpenAPIPatterns(t *testing.T) {
	doc := loadDoc(t)
	schemas := sub(t, doc, "components", "schemas")
	idPatterns := map[string]string{
		"параметр id": sub(t, doc, "components", "parameters", "id", "schema")["pattern"].(string),
		"Created.id":  sub(t, schemas, "Created", "properties", "id")["pattern"].(string),
		"Album.id":    sub(t, schemas, "Album", "properties", "id")["pattern"].(string),
	}
	pad := strings.Repeat("a", MinID)
	ids := []string{pad, "-" + pad, "_" + pad, pad + "-", "A" + strings.Repeat("-_", 11), pad + ".", pad + "/", pad + "%2e", pad + " ", pad + "\n",
		pad + "я", "..", pad + "\x00", strings.Repeat("Z9", 20)}
	for what, p := range idPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			t.Fatalf("%s: pattern %q: %v", what, p, err)
		}
		for _, id := range ids {
			if re.MatchString(id) != (CheckID(id) == nil) {
				t.Errorf("%s: pattern и CheckID расходятся на %q", what, id)
			}
		}
	}
	sha := sub(t, schemas, "Page", "properties", "sha256")["pattern"].(string)
	re := regexp.MustCompile(sha)
	h := strings.Repeat("0123456789abcdef", 4)
	for _, s := range []string{h, strings.ToUpper(h), h[1:], h + "0", h[:63] + "g", h[:63] + "\n", "", h[:63] + "я"} {
		if re.MatchString(s) != isSHA256(s) {
			t.Errorf("sha256: pattern и isSHA256 расходятся на %q", s)
		}
	}
}

// refName — имя схемы из {$ref: "#/components/schemas/X"} или "".
func refName(v any) string {
	mm, _ := v.(m)
	ref, _ := mm["$ref"].(string)
	return ref[strings.LastIndex(ref, "/")+1:]
}

// TestOpenAPIOperationTypes: схема тела запроса и 2xx-ответа каждой операции — тип, который берёт и
// возвращает метод Client с тем же именем.
func TestOpenAPIOperationTypes(t *testing.T) {
	doc := loadDoc(t)
	ct := reflect.TypeFor[*Client]()
	// ожидание: operationId → {схема тела, схема ответа}; "" — тела нет
	want := map[string][2]string{
		"Health": {"", "Health"}, "List": {"", "AlbumList"}, "Create": {"CreateRequest", "Created"},
		"Update": {"UpdateRequest", "Updated"}, "Patch": {"Patch", "Album"}, "Delete": {"", ""},
		"PutPage": {"", ""}, "Publish": {"", "Published"},
	}
	for path, item := range sub(t, doc, "paths") {
		for method, raw := range item.(m) {
			op, ok := raw.(m)
			if !ok || method == "parameters" {
				continue
			}
			id := op["operationId"].(string)
			gotReq, gotResp := opSchemas(t, op)
			if exp := want[id]; gotReq != exp[0] || gotResp != exp[1] {
				t.Errorf("%s %s: тело %q, ответ %q; ожидалось %q и %q", method, path, gotReq, gotResp, exp[0], exp[1])
			}
			meth, ok := ct.MethodByName(id)
			if !ok {
				continue // отсутствие метода ловит TestOpenAPIPaths
			}
			checkMethodTypes(t, id, meth.Type, gotReq, gotResp)
		}
	}
}

// opSchemas — имена схем тела запроса (JSON) и 2xx-ответа операции; "" — нет.
func opSchemas(t *testing.T, op m) (req, resp string) {
	t.Helper()
	if rb, ok := op["requestBody"].(m); ok {
		if js, ok := sub(t, rb, "content")["application/json"].(m); ok {
			req = refName(js["schema"])
		}
	}
	for code, r := range sub(t, op, "responses") {
		if content, ok := r.(m)["content"].(m); strings.HasPrefix(code, "2") && ok {
			resp = refName(content["application/json"].(m)["schema"])
		}
	}
	return req, resp
}

// checkMethodTypes сверяет аргумент-тело и результат метода со схемами.
func checkMethodTypes(t *testing.T, id string, mt reflect.Type, req, resp string) {
	t.Helper()
	deref := func(x reflect.Type) reflect.Type {
		for x.Kind() == reflect.Pointer {
			x = x.Elem()
		}
		return x
	}
	if req != "" {
		in := deref(mt.In(mt.NumIn() - 1)) // у Create/Update/Patch тело — последний аргумент
		switch id {
		case "Update": // метод берёт сам манифест, обёртка UpdateRequest — только на проводе
			f, ok := schemaTypes[req].FieldByName("Manifest")
			if !ok || f.Type != in {
				t.Errorf("%s: аргумент %v, UpdateRequest.Manifest %v", id, in, f.Type)
			}
		default:
			if in != schemaTypes[req] {
				t.Errorf("%s: аргумент %v, схема %s (%v)", id, in, req, schemaTypes[req])
			}
		}
	}
	if resp != "" {
		out := deref(mt.Out(0))
		if id == "List" { // []Album — поле albums схемы AlbumList
			out = reflect.TypeFor[AlbumList]()
		}
		if out != schemaTypes[resp] {
			t.Errorf("%s: результат %v, схема %s (%v)", id, out, resp, schemaTypes[resp])
		}
		if id == "List" && mt.Out(0) != reflect.TypeFor[[]Album]() {
			t.Errorf("List возвращает %v, нужен []Album", mt.Out(0))
		}
	} else if mt.NumOut() != 1 {
		t.Errorf("%s: 2xx без тела, а метод возвращает %d значений", id, mt.NumOut())
	}
}
