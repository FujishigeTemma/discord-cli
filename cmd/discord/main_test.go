package main

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

type recorded struct {
	Path   string
	Body   []byte
	Header http.Header
}

// fakeDiscord answers every request with the response for "METHOD path".
type fakeDiscord struct {
	responses map[string]string
	requests  []recorded
}

func (f *fakeDiscord) RoundTrip(r *http.Request) (*http.Response, error) {
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
	}
	path := strings.TrimPrefix(r.URL.EscapedPath(), "/api/v9")
	f.requests = append(f.requests, recorded{path, body, r.Header})
	resp, ok := f.responses[r.Method+" "+path]
	status := 200
	if !ok {
		status, resp = 404, `{"message":"Unknown Message","code":10008}`
	} else if resp == "" {
		status = 204
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(resp)),
		Request:    r,
	}, nil
}

// TestMain points config and cache at a temporary directory for all tests.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "discord-cli-test")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	os.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// run executes the CLI with args against fake and returns stdout lines.
func run(t *testing.T, fake *fakeDiscord, input string, args ...string) ([]map[string]any, error) {
	t.Helper()
	seenUsers = map[string]string{}
	if err := saveConfig(&Config{Token: "test-token", GuildID: "g1"}); err != nil {
		t.Fatal(err)
	}
	httpClient = &http.Client{Transport: fake}
	var out bytes.Buffer
	stdout, stdin = &out, strings.NewReader(input)

	parser, err := newParser()
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := parser.Parse(args)
	if err == nil {
		err = runCommand(ctx)
	}
	var lines []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if jerr := json.Unmarshal([]byte(l), &m); jerr != nil {
			t.Fatalf("stdout line is not JSON: %q", l)
		}
		lines = append(lines, m)
	}
	return lines, err
}

func msg(id, content, ts string) string {
	return `{"id":"` + id + `","channel_id":"c1","content":"` + content + `","timestamp":"` + ts +
		`","author":{"id":"u1","username":"alice","bot":false},"attachments":[],"reactions":[{"count":2,"me":true,"emoji":{"id":"9","name":"party"}}],"message_reference":{"message_id":"m0"}}`
}

func TestMessageListOldestFirst(t *testing.T) {
	fake := &fakeDiscord{responses: map[string]string{
		"GET /channels/c1/messages": "[" + msg("m3", "c", "2026-01-03T00:00:00Z") + "," +
			msg("m2", "b", "2026-01-02T00:00:00Z") + "," + msg("m1", "a", "2026-01-01T00:00:00Z") + "]",
	}}
	lines, err := run(t, fake, "", "message", "list", "c1", "--limit", "3", "--around", "m2")
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 || lines[0]["id"] != "m1" || lines[2]["id"] != "m3" {
		t.Fatalf("wrong order: %v", lines)
	}
	for _, k := range []string{"channel_id", "edited_timestamp", "attachments"} {
		if _, ok := lines[0][k]; ok {
			t.Errorf("%s should be omitted: %v", k, lines[0])
		}
	}
	if lines[0]["reply_to"] != "m0" {
		t.Errorf("reply_to = %v", lines[0]["reply_to"])
	}
	if r := lines[0]["reactions"].([]any)[0].(map[string]any); r["emoji"] != "party:9" || r["me"] != true {
		t.Errorf("emoji = %v", r["emoji"])
	}
}

func TestSendShortTextAndReply(t *testing.T) {
	fake := &fakeDiscord{responses: map[string]string{
		"POST /channels/c1/messages": msg("m9", "hi", "2026-01-01T00:00:00Z"),
	}}
	lines, err := run(t, fake, "hi\n", "message", "send", "c1", "--stdin", "--reply-to", "m0")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	json.Unmarshal(fake.requests[0].Body, &body)
	if body["content"] != "hi" {
		t.Errorf("content = %v", body["content"])
	}
	if ref := body["message_reference"].(map[string]any); ref["message_id"] != "m0" {
		t.Errorf("reference = %v", ref)
	}
	if _, ok := lines[0]["fallback"]; ok {
		t.Error("unexpected fallback")
	}
}

func TestSendLongTextFallsBackToAttachment(t *testing.T) {
	fake := &fakeDiscord{responses: map[string]string{
		"POST /channels/c1/messages": msg("m9", "", "2026-01-01T00:00:00Z"),
	}}
	long := strings.Repeat("あ", 2001)
	lines, err := run(t, fake, "", "message", "send", "c1", "--text", long)
	if err != nil {
		t.Fatal(err)
	}
	if lines[0]["fallback"] != "attachment" {
		t.Errorf("fallback = %v", lines[0]["fallback"])
	}
	req := fake.requests[0]
	_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	mr := multipart.NewReader(bytes.NewReader(req.Body), params["boundary"])
	var gotFile string
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		b, _ := io.ReadAll(p)
		if p.FileName() == "message.txt" {
			gotFile = string(b)
		}
		if p.FormName() == "payload_json" && strings.Contains(string(b), "あ") {
			t.Error("long text should not be in content")
		}
	}
	if gotFile != long {
		t.Errorf("attachment has %d bytes, want %d", len(gotFile), len(long))
	}
}

func TestSendNeedsSomething(t *testing.T) {
	_, err := run(t, &fakeDiscord{}, "", "message", "send", "c1")
	if err == nil || !strings.Contains(err.Error(), "nothing to send") {
		t.Fatalf("err = %v", err)
	}
}

func TestEditRejectsLongText(t *testing.T) {
	_, err := run(t, &fakeDiscord{}, "", "message", "edit", "c1", "m1", "--text", strings.Repeat("x", 2001))
	if err == nil || !strings.Contains(err.Error(), "at most 2000") {
		t.Fatalf("err = %v", err)
	}
}

func TestReactionEmojiForms(t *testing.T) {
	for _, tc := range []struct{ in, wantPath string }{
		{"👍", "/channels/c1/messages/m1/reactions/%F0%9F%91%8D/@me"},
		{"party:9", "/channels/c1/messages/m1/reactions/party:9/@me"},
	} {
		fake := &fakeDiscord{responses: map[string]string{"PUT " + tc.wantPath: ""}}
		lines, err := run(t, fake, "", "reaction", "add", "c1", "m1", tc.in)
		if err != nil {
			t.Fatalf("%s: %v (path %s)", tc.in, err, fake.requests[0].Path)
		}
		if lines[0]["ok"] != true {
			t.Errorf("%s: %v", tc.in, lines)
		}
	}
}

func TestThreadListFilter(t *testing.T) {
	fake := &fakeDiscord{responses: map[string]string{
		"GET /guilds/g1/threads/active": `{"threads":[
			{"id":"20","name":"b","type":11,"parent_id":"c1","thread_metadata":{"archived":false}},
			{"id":"3","name":"a","type":11,"parent_id":"c1"},
			{"id":"4","name":"x","type":11,"parent_id":"c2"}],"members":[]}`,
	}}
	lines, err := run(t, fake, "", "thread", "list", "--channel", "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || lines[0]["id"] != "20" || lines[1]["type"] != "public_thread" {
		t.Fatalf("lines = %v", lines)
	}
}

func TestForumThreadNeedsText(t *testing.T) {
	fake := &fakeDiscord{responses: map[string]string{
		"GET /channels/f1":          `{"id":"f1","type":15}`,
		"POST /channels/f1/threads": `{"id":"t1","name":"post","type":11,"parent_id":"f1"}`,
	}}
	if _, err := run(t, fake, "", "thread", "create", "f1", "--name", "post"); err == nil {
		t.Fatal("expected error without text")
	}
	lines, err := run(t, fake, "", "thread", "create", "f1", "--name", "post", "--text", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if lines[0]["id"] != "t1" {
		t.Fatalf("lines = %v", lines)
	}
	last := fake.requests[len(fake.requests)-1]
	if !strings.Contains(string(last.Body), `"content":"hello"`) {
		t.Errorf("body = %s", last.Body)
	}
}

func TestMissingSubcommandShowsHelp(t *testing.T) {
	for _, tc := range []struct {
		args []string
		help bool
	}{
		{[]string{}, true},
		{[]string{"message"}, true},
		{[]string{"message", "nope"}, true},
		{[]string{"message", "list"}, false}, // missing argument is a normal error
	} {
		parser, _ := newParser()
		_, err := parser.Parse(tc.args)
		var perr *kong.ParseError
		if !errors.As(err, &perr) {
			t.Fatalf("%v: err = %v", tc.args, err)
		}
		if got := hasSubcommands(perr.Context); got != tc.help {
			t.Errorf("%v: help = %v, want %v", tc.args, got, tc.help)
		}
	}
}

func TestTextThreadCreateSendsFirstMessage(t *testing.T) {
	fake := &fakeDiscord{responses: map[string]string{
		"GET /channels/c1":           `{"id":"c1","type":0}`,
		"POST /channels/c1/threads":  `{"id":"t1","name":"topic","type":11,"parent_id":"c1"}`,
		"POST /channels/t1/messages": msg("m1", "hello", "2026-01-01T00:00:00Z"),
	}}
	lines, err := run(t, fake, "", "thread", "create", "c1", "--name", "topic", "--text", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if lines[0]["id"] != "t1" || len(fake.requests) != 3 {
		t.Fatalf("lines = %v, requests = %d", lines, len(fake.requests))
	}

	// A failed create must surface as an error, not a nil thread.
	delete(fake.responses, "POST /channels/c1/threads")
	if _, err := run(t, fake, "", "thread", "create", "c1", "--name", "topic"); err == nil {
		t.Fatal("expected error")
	}
}

func TestReactionList(t *testing.T) {
	fake := &fakeDiscord{responses: map[string]string{
		"GET /channels/c1/messages/m1/reactions/%F0%9F%91%8D": `[{"id":"u1","username":"alice"},{"id":"b1","username":"bot","bot":true}]`,
	}}
	lines, err := run(t, fake, "", "reaction", "list", "c1", "m1", "👍")
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || lines[0]["name"] != "alice" || lines[1]["bot"] != true {
		t.Fatalf("lines = %v", lines)
	}
}

func TestMentionsByName(t *testing.T) {
	fake := &fakeDiscord{responses: map[string]string{
		"GET /channels/c1/messages/m1": `{"id":"m1","channel_id":"c1","content":"<@u2> hi","timestamp":"2026-01-01T00:00:00Z",
			"author":{"id":"u1","username":"alice"},"mentions":[{"id":"u2","username":"bob"}]}`,
		"POST /channels/c1/messages": msg("m2", "", "2026-01-01T00:00:00Z"),
	}}
	lines, err := run(t, fake, "", "message", "get", "c1", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if lines[0]["content"] != "<@bob> hi" {
		t.Errorf("content = %v", lines[0]["content"])
	}

	// Names seen above are cached and resolve to IDs.
	lines, err = run(t, fake, "", "message", "send", "c1", "--text", "hello", "--mention", "@bob", "--mention", "<@alice>")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	json.Unmarshal(fake.requests[len(fake.requests)-1].Body, &body)
	if body["content"] != "<@u2> <@u1> hello" {
		t.Errorf("content = %v", body["content"])
	}
	if len(lines[0]) != 1 || lines[0]["id"] != "m2" {
		t.Errorf("send output = %v, want only id", lines[0])
	}

	if _, err := run(t, fake, "", "message", "send", "c1", "--text", "x", "--mention", "carol"); err == nil {
		t.Error("expected unknown user error")
	}
}

func TestAttachmentDownload(t *testing.T) {
	fake := &fakeDiscord{responses: map[string]string{
		"GET /channels/c1/messages/m1": `{"id":"m1","channel_id":"c1","timestamp":"2026-01-01T00:00:00Z","author":{"id":"u1","username":"alice"},
			"attachments":[{"id":"a1","filename":"shot.png","url":"https://cdn.discordapp.com/attachments/c1/a1/shot.png?ex=1"}]}`,
		"GET /attachments/c1/a1/shot.png": "PNGDATA",
	}}
	lines, err := run(t, fake, "", "message", "get", "c1", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if a := lines[0]["attachments"].([]any); a[0] != "shot.png" {
		t.Errorf("attachments = %v", a)
	}
	lines, err = run(t, fake, "", "attachment", "download", "c1", "m1")
	if err != nil {
		t.Fatal(err)
	}
	path := lines[0]["path"].(string)
	if b, _ := os.ReadFile(path); string(b) != "PNGDATA" {
		t.Errorf("file = %q", b)
	}
	if !strings.HasSuffix(path, filepath.Join("cache", "discord-cli", "attachments", "a1", "shot.png")) {
		t.Errorf("path = %s", path)
	}
}

func TestLongTextKeepsMentionsInContent(t *testing.T) {
	fake := &fakeDiscord{responses: map[string]string{
		"GET /channels/c1/messages/m1": `{"id":"m1","channel_id":"c1","timestamp":"2026-01-01T00:00:00Z","author":{"id":"u1","username":"alice"}}`,
		"POST /channels/c1/messages":   msg("m2", "", "2026-01-01T00:00:00Z"),
	}}
	if _, err := run(t, fake, "", "message", "get", "c1", "m1"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, fake, "", "message", "send", "c1", "--text", strings.Repeat("x", 2001), "--mention", "alice"); err != nil {
		t.Fatal(err)
	}
	if body := string(fake.requests[len(fake.requests)-1].Body); !strings.Contains(body, `"content":"\u003c@u1\u003e "`) {
		t.Errorf("mention should stay in content: %.300s", body)
	}
}

func TestAttachmentDownloadSameNames(t *testing.T) {
	fake := &fakeDiscord{responses: map[string]string{
		"GET /channels/c1/messages/m1": `{"id":"m1","channel_id":"c1","timestamp":"2026-01-01T00:00:00Z","author":{"id":"u1","username":"alice"},
			"attachments":[{"id":"a1","filename":"same.txt","url":"https://cdn.discordapp.com/x/a1"},{"id":"a2","filename":"same.txt","url":"https://cdn.discordapp.com/x/a2"}]}`,
		"GET /x/a1": "one",
		"GET /x/a2": "two",
	}}
	lines, err := run(t, fake, "", "attachment", "download", "c1", "m1")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(lines[0]["path"].(string))
	b, _ := os.ReadFile(lines[1]["path"].(string))
	if string(a) != "one" || string(b) != "two" {
		t.Errorf("got %q and %q", a, b)
	}
}
