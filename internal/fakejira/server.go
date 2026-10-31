// Package fakejira is an in-memory fake of the parts of the Jira Cloud REST
// API that jt uses. It is seeded with demo data and keeps changes (moves,
// assignments, comments and edits) until it stops, so jt can be tried and
// tested without a Jira site.
package fakejira

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const timeLayout = "2006-01-02T15:04:05.000-0700"

// Server serves the fake Jira API. Every request needs basic auth, but any
// login and token are accepted.
type Server struct {
	mux *http.ServeMux

	mu       sync.Mutex
	now      func() time.Time
	users    []user
	projects []project
	boards   []board
	fields   []fieldDef
	issues   []*issue
	nextID   int
}

// New returns a server seeded with demo projects, boards and issues.
func New() *Server {
	s := &Server{
		now:      time.Now,
		users:    seedUsers(),
		projects: seedProjects(),
		boards:   seedBoards(),
		fields:   fieldDefs(),
		nextID:   5000,
	}
	s.issues = seedIssues(s.now())

	m := http.NewServeMux()
	m.HandleFunc("GET /rest/api/2/myself", s.myself)
	m.HandleFunc("GET /rest/api/2/project", s.listProjects)
	m.HandleFunc("GET /rest/api/2/issue/{key}", s.editMeta)
	m.HandleFunc("PUT /rest/api/2/issue/{key}", s.edit)
	m.HandleFunc("POST /rest/api/2/issue/{key}/transitions", s.transition)
	m.HandleFunc("POST /rest/api/2/issue/{key}/comment", s.addComment)
	m.HandleFunc("GET /rest/api/3/issue/{key}", s.issueDetail)
	m.HandleFunc("GET /rest/api/3/issue/{key}/transitions", s.transitions)
	m.HandleFunc("PUT /rest/api/3/issue/{key}/assignee", s.assign)
	m.HandleFunc("GET /rest/api/3/user/assignable/search", s.searchUsers)
	m.HandleFunc("GET /rest/agile/1.0/board", s.listBoards)
	m.HandleFunc("GET /rest/agile/1.0/board/{id}", s.getBoard)
	m.HandleFunc("GET /rest/agile/1.0/board/{id}/configuration", s.boardConfig)
	m.HandleFunc("GET /rest/agile/1.0/board/{id}/issue", s.boardIssues)
	m.HandleFunc("GET /rest/agile/1.0/board/{id}/quickfilter", s.quickFilters)
	m.HandleFunc("GET /browse/{key}", s.browse)
	s.mux = m
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/browse/") {
		if _, _, ok := r.BasicAuth(); !ok {
			writeError(w, http.StatusUnauthorized, "Client must be authenticated to access this resource.")
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mux.ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError replies with Jira's error body.
func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"errorMessages": []string{msg}, "errors": map[string]string{}})
}

func writeFieldError(w http.ResponseWriter, field, msg string) {
	writeJSON(w, http.StatusBadRequest, map[string]any{"errorMessages": []string{}, "errors": map[string]string{field: msg}})
}

func (s *Server) user(id string) *user {
	for i := range s.users {
		if s.users[i].AccountID == id {
			return &s.users[i]
		}
	}
	return nil
}

func (s *Server) userJSON(id string) any {
	u := s.user(id)
	if u == nil {
		return nil
	}
	return map[string]any{"accountId": u.AccountID, "displayName": u.Name, "emailAddress": u.Email, "active": u.Active}
}

func (s *Server) findIssue(w http.ResponseWriter, r *http.Request) *issue {
	key := r.PathValue("key")
	for _, iss := range s.issues {
		if strings.EqualFold(iss.Key, key) {
			return iss
		}
	}
	writeError(w, http.StatusNotFound, "Issue does not exist or you do not have permission to see it.")
	return nil
}

func (s *Server) findBoard(w http.ResponseWriter, r *http.Request) *board {
	id, _ := strconv.Atoi(r.PathValue("id"))
	for i := range s.boards {
		if s.boards[i].ID == id {
			return &s.boards[i]
		}
	}
	writeError(w, http.StatusNotFound, "The requested board cannot be viewed because it either does not exist or you do not have permission to view it.")
	return nil
}

func statusByID(id string) status {
	for _, st := range allStatuses {
		if st.ID == id {
			return st
		}
	}
	return status{ID: id, Name: id}
}

func (s *Server) myself(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"accountId": "user-sam", "displayName": "Sam Tester", "emailAddress": Login, "timeZone": "UTC", "active": true,
	})
}

func (s *Server) listProjects(w http.ResponseWriter, _ *http.Request) {
	out := make([]any, 0, len(s.projects))
	for _, p := range s.projects {
		out = append(out, map[string]any{
			"key": p.Key, "name": p.Name, "style": "next-gen",
			"lead": map[string]any{"displayName": "Ada Lovelace"},
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func boardJSON(b *board) map[string]any {
	return map[string]any{"id": b.ID, "name": b.Name, "type": b.Type, "location": map[string]any{"projectKey": b.Project}}
}

func (s *Server) listBoards(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("projectKeyOrId")
	values := []any{}
	for i := range s.boards {
		if project == "" || strings.EqualFold(s.boards[i].Project, project) {
			values = append(values, boardJSON(&s.boards[i]))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"values": values, "isLast": true, "total": len(values)})
}

func (s *Server) getBoard(w http.ResponseWriter, r *http.Request) {
	if b := s.findBoard(w, r); b != nil {
		writeJSON(w, http.StatusOK, boardJSON(b))
	}
}

func (s *Server) boardConfig(w http.ResponseWriter, r *http.Request) {
	b := s.findBoard(w, r)
	if b == nil {
		return
	}
	cols := make([]any, 0, len(b.Columns))
	for _, c := range b.Columns {
		sts := make([]any, 0, len(c.Statuses))
		for _, id := range c.Statuses {
			sts = append(sts, map[string]any{"id": id})
		}
		cols = append(cols, map[string]any{"name": c.Name, "statuses": sts})
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": b.ID, "name": b.Name, "columnConfig": map[string]any{"columns": cols}})
}

func (s *Server) quickFilters(w http.ResponseWriter, r *http.Request) {
	b := s.findBoard(w, r)
	if b == nil {
		return
	}
	values := []any{}
	for i, q := range b.Quick {
		values = append(values, map[string]any{"id": i + 1, "boardId": b.ID, "name": q.Name, "jql": q.JQL})
	}
	writeJSON(w, http.StatusOK, map[string]any{"values": values, "isLast": true})
}

func (s *Server) boardIssues(w http.ResponseWriter, r *http.Request) {
	b := s.findBoard(w, r)
	if b == nil {
		return
	}
	q := r.URL.Query()
	match := jqlMatcher(q.Get("jql"))
	var matched []*issue
	for _, iss := range s.issues {
		if strings.EqualFold(iss.project(), b.Project) && match(iss) {
			matched = append(matched, iss)
		}
	}
	start, _ := strconv.Atoi(q.Get("startAt"))
	maxResults, err := strconv.Atoi(q.Get("maxResults"))
	if err != nil || maxResults <= 0 {
		maxResults = 50
	}
	start = min(max(start, 0), len(matched))
	page := matched[start:min(start+maxResults, len(matched))]

	issues := make([]any, 0, len(page))
	for _, iss := range page {
		st := statusByID(iss.StatusID)
		issues = append(issues, map[string]any{
			"key": iss.Key,
			"fields": map[string]any{
				"summary":   iss.Fields["summary"],
				"status":    map[string]any{"id": st.ID, "name": st.Name},
				"assignee":  s.userJSON(iss.AssigneeID),
				"issuetype": map[string]any{"name": iss.Type},
				"priority":  iss.Fields["priority"],
			},
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"startAt": start, "maxResults": maxResults, "total": len(matched), "issues": issues})
}

var (
	jqlAssigneeRe = regexp.MustCompile(`(?i)\bassignee\s*=\s*"?([^"\s)]+)"?`)
	jqlEmptyRe    = regexp.MustCompile(`(?i)\bassignee\s+is\s+EMPTY\b`)
	jqlTypeRe     = regexp.MustCompile(`(?i)\bissuetype\s*=\s*"?([^"\s)]+)"?`)
	jqlLabelRe    = regexp.MustCompile(`(?i)\blabels\s*=\s*"?([^"\s)]+)"?`)
)

// jqlMatcher understands just enough JQL for jt's filters: "assignee = x",
// "assignee is EMPTY", "issuetype = x" and "labels = x", all ANDed together.
// Everything else, such as the board's own open-work clause, is ignored.
func jqlMatcher(jql string) func(*issue) bool {
	var conds []func(*issue) bool
	for _, m := range jqlAssigneeRe.FindAllStringSubmatch(jql, -1) {
		id := m[1]
		conds = append(conds, func(i *issue) bool { return i.AssigneeID == id })
	}
	if jqlEmptyRe.MatchString(jql) {
		conds = append(conds, func(i *issue) bool { return i.AssigneeID == "" })
	}
	for _, m := range jqlTypeRe.FindAllStringSubmatch(jql, -1) {
		typ := m[1]
		conds = append(conds, func(i *issue) bool { return strings.EqualFold(i.Type, typ) })
	}
	for _, m := range jqlLabelRe.FindAllStringSubmatch(jql, -1) {
		label := m[1]
		conds = append(conds, func(i *issue) bool {
			return slices.Contains(i.labels(), any(label))
		})
	}
	return func(i *issue) bool {
		for _, c := range conds {
			if !c(i) {
				return false
			}
		}
		return true
	}
}

func (s *Server) issueDetail(w http.ResponseWriter, r *http.Request) {
	iss := s.findIssue(w, r)
	if iss == nil {
		return
	}
	var desc any
	if d, _ := iss.Fields["description"].(string); d != "" {
		desc = toADF(d)
	}
	comments := make([]any, 0, len(iss.Comments))
	for _, c := range iss.Comments {
		comments = append(comments, map[string]any{
			"id": c.ID, "author": s.userJSON(c.AuthorID), "body": toADF(c.Body), "created": c.Created.Format(timeLayout),
		})
	}
	st := statusByID(iss.StatusID)
	writeJSON(w, http.StatusOK, map[string]any{
		"key": iss.Key,
		"fields": map[string]any{
			"summary":     iss.Fields["summary"],
			"description": desc,
			"labels":      iss.labels(),
			"issuetype":   map[string]any{"name": iss.Type},
			"status":      map[string]any{"id": st.ID, "name": st.Name},
			"assignee":    s.userJSON(iss.AssigneeID),
			"reporter":    s.userJSON(iss.ReporterID),
			"priority":    iss.Fields["priority"],
			"comment":     map[string]any{"comments": comments, "total": len(comments), "maxResults": len(comments), "startAt": 0},
			"created":     iss.Created.Format(timeLayout),
			"updated":     iss.Updated.Format(timeLayout),
		},
	})
}

func (s *Server) transitions(w http.ResponseWriter, r *http.Request) {
	iss := s.findIssue(w, r)
	if iss == nil {
		return
	}
	out := []any{}
	for _, st := range allStatuses {
		if st.ID == iss.StatusID {
			continue
		}
		out = append(out, map[string]any{"id": st.ID, "name": st.Name, "to": map[string]any{"id": st.ID, "name": st.Name}})
	}
	writeJSON(w, http.StatusOK, map[string]any{"transitions": out})
}

func (s *Server) transition(w http.ResponseWriter, r *http.Request) {
	iss := s.findIssue(w, r)
	if iss == nil {
		return
	}
	var req struct {
		Transition struct {
			ID string `json:"id"`
		} `json:"transition"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload.")
		return
	}
	id := req.Transition.ID
	if id == iss.StatusID || !slices.ContainsFunc(allStatuses, func(st status) bool { return st.ID == id }) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Transition id '%s' is not valid for this issue.", id))
		return
	}
	iss.StatusID = id
	iss.Updated = s.now()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) assign(w http.ResponseWriter, r *http.Request) {
	iss := s.findIssue(w, r)
	if iss == nil {
		return
	}
	var req struct {
		AccountID *string `json:"accountId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload.")
		return
	}
	switch {
	case req.AccountID == nil || *req.AccountID == "-1":
		iss.AssigneeID = ""
	case s.user(*req.AccountID) == nil || !s.user(*req.AccountID).Active:
		writeFieldError(w, "assignee", fmt.Sprintf("User '%s' cannot be assigned issues.", *req.AccountID))
		return
	default:
		iss.AssigneeID = *req.AccountID
	}
	iss.Updated = s.now()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) addComment(w http.ResponseWriter, r *http.Request) {
	iss := s.findIssue(w, r)
	if iss == nil {
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Body) == "" {
		writeFieldError(w, "comment", "Comment body can not be empty!")
		return
	}
	s.nextID++
	c := comment{ID: strconv.Itoa(s.nextID), AuthorID: "user-sam", Body: req.Body, Created: s.now()}
	iss.Comments = append(iss.Comments, c)
	iss.Updated = c.Created
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": c.ID, "author": s.userJSON(c.AuthorID), "body": c.Body, "created": c.Created.Format(timeLayout),
	})
}

func (s *Server) searchUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := strings.ToLower(q.Get("query"))
	limit, err := strconv.Atoi(q.Get("maxResults"))
	if err != nil || limit <= 0 {
		limit = 50
	}
	out := []any{}
	for _, u := range s.users {
		if len(out) == limit {
			break
		}
		if strings.Contains(strings.ToLower(u.Name), query) || strings.Contains(strings.ToLower(u.Email), query) {
			out = append(out, s.userJSON(u.AccountID))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) fieldDef(id string) *fieldDef {
	for i := range s.fields {
		if s.fields[i].ID == id {
			return &s.fields[i]
		}
	}
	return nil
}

// editMeta serves the v2 issue, which jt reads for editing. Only the editable
// fields are returned, narrowed by the fields parameter when given.
func (s *Server) editMeta(w http.ResponseWriter, r *http.Request) {
	iss := s.findIssue(w, r)
	if iss == nil {
		return
	}
	q := r.URL.Query()
	var want []string
	if f := q.Get("fields"); f != "" {
		want = strings.Split(f, ",")
	}
	fields := map[string]any{}
	for _, def := range s.fields {
		if want == nil || slices.Contains(want, def.ID) {
			fields[def.ID] = iss.Fields[def.ID]
		}
	}
	res := map[string]any{"key": iss.Key, "fields": fields}

	if slices.Contains(strings.Split(q.Get("expand"), ","), "editmeta") {
		meta := map[string]any{}
		for _, def := range s.fields {
			schema := map[string]any{"type": def.Type}
			if def.Items != "" {
				schema["items"] = def.Items
			}
			if def.Custom != "" {
				schema["custom"] = def.Custom
			} else {
				schema["system"] = def.ID
			}
			m := map[string]any{
				"required": def.Required, "name": def.Name, "key": def.ID, "schema": schema,
				"operations": []string{"set"},
			}
			if def.Allowed != nil {
				allowed := make([]any, 0, len(def.Allowed))
				for _, a := range def.Allowed {
					allowed = append(allowed, a.json())
				}
				m["allowedValues"] = allowed
			}
			meta[def.ID] = m
		}
		res["editmeta"] = map[string]any{"fields": meta}
	}
	writeJSON(w, http.StatusOK, res)
}

// edit applies a v2 edit. Values are checked against the field definitions
// and stored the way Jira would return them.
func (s *Server) edit(w http.ResponseWriter, r *http.Request) {
	iss := s.findIssue(w, r)
	if iss == nil {
		return
	}
	var req struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload.")
		return
	}
	updates := make(map[string]any, len(req.Fields))
	for id, raw := range req.Fields {
		def := s.fieldDef(id)
		if def == nil {
			writeFieldError(w, id, fmt.Sprintf("Field '%s' cannot be set. It is not on the appropriate screen, or unknown.", id))
			return
		}
		v, err := s.fieldValue(def, raw)
		if err != nil {
			writeFieldError(w, id, err.Error())
			return
		}
		updates[id] = v
	}
	for id, v := range updates {
		iss.Fields[id] = v
	}
	iss.Updated = s.now()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) fieldValue(def *fieldDef, raw json.RawMessage) (any, error) {
	if string(raw) == "null" || len(raw) == 0 {
		if def.Required {
			return nil, fmt.Errorf("you must specify a %s", strings.ToLower(def.Name))
		}
		return nil, nil
	}
	type ref struct {
		ID        string `json:"id"`
		AccountID string `json:"accountId"`
	}
	allowed := func(id string) (any, error) {
		for _, a := range def.Allowed {
			if a.ID == id {
				return a.json(), nil
			}
		}
		return nil, fmt.Errorf("specify a valid value for %s", def.Name)
	}
	user := func(id string) (any, error) {
		if u := s.user(id); u != nil {
			return s.userJSON(id), nil
		}
		return nil, fmt.Errorf("user %q does not exist", id)
	}
	var one ref
	var many []ref
	switch {
	case def.Type == "option" || def.Type == "priority":
		if err := json.Unmarshal(raw, &one); err != nil {
			return nil, err
		}
		return allowed(one.ID)
	case def.Type == "user":
		if err := json.Unmarshal(raw, &one); err != nil {
			return nil, err
		}
		return user(one.AccountID)
	case def.Type == "array" && def.Items != "string":
		if err := json.Unmarshal(raw, &many); err != nil {
			return nil, err
		}
		out := make([]any, 0, len(many))
		for _, r := range many {
			var v any
			var err error
			if def.Items == "user" {
				v, err = user(r.AccountID)
			} else {
				v, err = allowed(r.ID)
			}
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	if def.Required {
		if str, ok := v.(string); ok && strings.TrimSpace(str) == "" {
			return nil, fmt.Errorf("you must specify a %s", strings.ToLower(def.Name))
		}
	}
	return v, nil
}

var browseTmpl = template.Must(template.New("browse").Parse(`<!doctype html>
<title>{{.Key}}</title>
<h1>{{.Key}}: {{.Summary}}</h1>
<p>This is the fake Jira server. The issue lives in jt.</p>
`))

func (s *Server) browse(w http.ResponseWriter, r *http.Request) {
	iss := s.findIssue(w, r)
	if iss == nil {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = browseTmpl.Execute(w, map[string]any{"Key": iss.Key, "Summary": iss.Fields["summary"]})
}
