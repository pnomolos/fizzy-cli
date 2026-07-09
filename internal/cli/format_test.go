package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

// renderTable runs a row function and renders the resulting table (with header,
// non-plain) exactly as the commands do.
func renderTable(t *testing.T, headers []string, rowFn func([]byte) ([][]string, error), body string) string {
	t.Helper()
	rows, err := rowFn([]byte(body))
	if err != nil {
		t.Fatalf("row fn: %v", err)
	}
	var buf bytes.Buffer
	printTable(&buf, headers, rows, false)
	return buf.String()
}

const (
	boardsFixture = `[{"id":"b1","name":"Roadmap","all_access":true,"created_at":"2024-01-02"},{"id":"b2","name":"Bugs","all_access":false,"created_at":"2024-03-04"}]`
	cardsFixture  = `[{"number":7,"title":"Fix login","status":"open","board":{"name":"Bugs"},"last_active_at":"2024-05-06"},{"number":12,"title":"Add search","status":"closed","board":{"name":"Roadmap"},"last_active_at":"2024-05-07"}]`
)

func TestBoardListRowsGolden(t *testing.T) {
	want := "ID  NAME     ALL_ACCESS  CREATED\nb1  Roadmap  true        2024-01-02\nb2  Bugs     false       2024-03-04\n"
	if got := renderTable(t, boardListHeaders, boardListRows, boardsFixture); got != want {
		t.Errorf("board table mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestCardListRowsGolden(t *testing.T) {
	want := "#   TITLE       STATUS  BOARD    LAST_ACTIVE\n7   Fix login   open    Bugs     2024-05-06\n12  Add search  closed  Roadmap  2024-05-07\n"
	if got := renderTable(t, cardListHeaders, cardListRows, cardsFixture); got != want {
		t.Errorf("card table mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestCommentListRowsGolden(t *testing.T) {
	body := `[{"id":"c1","creator":{"name":"Ada"},"body":{"plain_text":"Looks good"},"created_at":"2024-06-01"}]`
	want := "ID  CREATOR  BODY        CREATED\nc1  Ada      Looks good  2024-06-01\n"
	if got := renderTable(t, commentListHeaders, commentListRows, body); got != want {
		t.Errorf("comment table mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestTagListRowsGolden(t *testing.T) {
	body := `[{"id":"t1","title":"urgent"},{"id":"t2","title":"backend"}]`
	want := "ID  TITLE\nt1  urgent\nt2  backend\n"
	if got := renderTable(t, tagListHeaders, tagListRows, body); got != want {
		t.Errorf("tag table mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestUserListRowsGolden(t *testing.T) {
	body := `[{"id":"u1","name":"Ada Lovelace","role":"admin","email_address":"ada@example.com"}]`
	want := "ID  NAME          ROLE   EMAIL\nu1  Ada Lovelace  admin  ada@example.com\n"
	if got := renderTable(t, userListHeaders, userListRows, body); got != want {
		t.Errorf("user table mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestNotificationListRowsGolden(t *testing.T) {
	body := `[{"id":"n1","read":false,"title":"Assigned","card":{"title":"Fix login"},"created_at":"2024-07-01"},{"id":"n2","read":true,"title":"Mentioned","card":{"title":"Add search"},"created_at":"2024-07-02"}]`
	want := "ID  READ  TITLE      CARD        CREATED\nn1  no    Assigned   Fix login   2024-07-01\nn2  yes   Mentioned  Add search  2024-07-02\n"
	if got := renderTable(t, notificationListHeaders, notificationListRows, body); got != want {
		t.Errorf("notification table mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestIdentityToRowsGolden(t *testing.T) {
	body := `{"accounts":[{"slug":"/acme","name":"Acme Inc","user":{"name":"Ada"}},{"slug":"globex","name":"Globex","user":{"name":"Bob"}}]}`
	want := "SLUG    NAME      USER\nacme    Acme Inc  Ada\nglobex  Globex    Bob\n"
	if got := renderTable(t, []string{"SLUG", "NAME", "USER"}, identityToRows, body); got != want {
		t.Errorf("identity table mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestFormatBoard(t *testing.T) {
	body := `{"id":"b1","name":"Roadmap","all_access":true,"created_at":"2024-01-02","creator":{"name":"Ada"},"url":"https://app/x"}`
	want := "ID: b1\nName: Roadmap\nAll access: true\nCreated: 2024-01-02\nCreator: Ada\nURL: https://app/x"
	got, err := formatBoard([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("formatBoard\n got: %q\nwant: %q", got, want)
	}
}

func TestFormatCard(t *testing.T) {
	body := `{"id":"cardid","number":7,"title":"Fix login","status":"open","board":{"name":"Bugs"},"creator":{"name":"Ada"},"tags":["urgent","backend"],"golden":true,"last_active_at":"2024-05-06","created_at":"2024-05-01","description":"Details here","steps":[{"content":"Repro","completed":true},{"content":"Patch","completed":false}]}`
	want := "ID: cardid\n" +
		"Number: 7\n" +
		"Title: Fix login\n" +
		"Status: open\n" +
		"Board: Bugs\n" +
		"Creator: Ada\n" +
		"Tags: urgent, backend\n" +
		"Golden: true\n" +
		"Last active: 2024-05-06\n" +
		"Created: 2024-05-01\n" +
		"\nDescription:\nDetails here\n" +
		"\nSteps:\n" +
		"- [x] Repro\n" +
		"- [ ] Patch"
	got, err := formatCard([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("formatCard\n got: %q\nwant: %q", got, want)
	}
}

func TestFormatCardMinimal(t *testing.T) {
	body := `{"id":"x","number":1,"title":"T","status":"open","board":{"name":"B"},"creator":{"name":"C"}}`
	want := "ID: x\nNumber: 1\nTitle: T\nStatus: open\nBoard: B\nCreator: C"
	got, err := formatCard([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("formatCard minimal\n got: %q\nwant: %q", got, want)
	}
}

func TestFormatComment(t *testing.T) {
	body := `{"id":"c1","creator":{"name":"Ada"},"created_at":"2024-06-01","body":{"plain_text":"Looks good"}}`
	want := "ID: c1\nCreator: Ada\nCreated: 2024-06-01\nBody: Looks good"
	got, err := formatComment([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("formatComment\n got: %q\nwant: %q", got, want)
	}
}

func TestColumnColorUnmarshal(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"null", `{"id":"c1","name":"Todo","color":null}`, ""},
		{"string", `{"id":"c1","name":"Todo","color":"Blue"}`, "Blue"},
		{"object", `{"id":"c1","name":"Todo","color":{"name":"Blue","value":"var(--color-card-default)"}}`, "Blue"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var col column
			if err := json.Unmarshal([]byte(tc.in), &col); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if col.Color.Name != tc.want {
				t.Errorf("color name = %q, want %q", col.Color.Name, tc.want)
			}
		})
	}
}

func TestColumnListRowsGolden(t *testing.T) {
	body := `[{"id":"c1","name":"Todo","color":{"name":"Blue","value":"var(--color-card-default)"}},{"id":"c2","name":"Done","color":null}]`
	want := "ID  NAME  COLOR\nc1  Todo  Blue\nc2  Done  \n"
	if got := renderTable(t, columnListHeaders, columnListRows, body); got != want {
		t.Errorf("column table mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestFormatColumnObjectColor(t *testing.T) {
	body := `{"id":"c1","name":"Todo","color":{"name":"Blue","value":"var(--color-card-default)"},"created_at":"2024-01-02"}`
	want := "ID: c1\nName: Todo\nColor: Blue\nCreated: 2024-01-02"
	got, err := formatColumn([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("formatColumn\n got: %q\nwant: %q", got, want)
	}
}

func TestFormatUser(t *testing.T) {
	body := `{"id":"u1","name":"Ada","role":"admin","email_address":"ada@example.com"}`
	want := "ID: u1\nName: Ada\nRole: admin\nEmail: ada@example.com"
	got, err := formatUser([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("formatUser\n got: %q\nwant: %q", got, want)
	}
}
