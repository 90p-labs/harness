package domain_test

import (
	"testing"

	"github.com/90p-labs/harness/internal/report/domain"
)

const userHome = "/Users/jorgearco"

func TestRedact(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"email", "commit by jorge.arcoma@gmail.com on main", "commit by <redacted> on main"},
		{"mac home path", "open /Users/jorge/work/harness/.harness.yaml", "open ~/work/harness/.harness.yaml"},
		{"linux home path", "cd /home/dev/shop && harness verify", "cd ~/shop && harness verify"},
		{"home path in JSON", `{"cwd":"/Users/jorgearco","hook_event_name":"Stop"}`, `{"cwd":"~","hook_event_name":"Stop"}`},
		{"home path of another user in JSON", `{"cwd":"/home/ci","hook_event_name":"Stop"}`, `{"cwd":"~","hook_event_name":"Stop"}`},
		{
			"encoded home path",
			`cd /private/tmp/claude-501/-Users-jorgearco-work-acme-app/x "transcript_path":"/Users/jorgearco/.claude/projects/-Users-jorgearco-work-acme-app/a.jsonl"`,
			`cd /private/tmp/claude-501/~-work-acme-app/x "transcript_path":"~/.claude/projects/~-work-acme-app/a.jsonl"`,
		},
		{"GitHub token", "GH_AUTH ghp_aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789 used", "GH_AUTH <redacted> used"},
		{"fine-grained GitHub token", "github_pat_11ABCDEFG0123456789_abcdefghijklmnop", "<redacted>"},
		{"API key", "export ANTHROPIC=sk-ant-api03-AbCdEfGhIjKlMnOpQrSt", "export ANTHROPIC=<redacted>"},
		{"AWS key", "key AKIAIOSFODNN7EXAMPLE in env", "key <redacted> in env"},
		{
			"Slack, GitLab and npm tokens",
			"xoxb-1234567890-abcdefghij glpat-abcdefghij0123456789 npm_abcdefghijklmnopqrstuvwxyz0123456789",
			"<redacted> <redacted> <redacted>",
		},
		{"bearer token", "Authorization: Bearer abc.def-123", "Authorization: Bearer <redacted>"},
		{"basic auth", "Authorization: Basic dXNlcjpwYXNzd29yZA==", "Authorization: Basic <redacted>"},
		{
			"quoted Authorization header",
			`{"Authorization": "Basic dXNlcjpwYXNz"} -H "Authorization: Basic dXNlcjpwYXNz"`,
			`{"Authorization": "Basic <redacted>"} -H "Authorization: Basic <redacted>"`,
		},
		{"lowercase bearer token", "using bearer 4f9c2a7e1b3d", "using bearer <redacted>"},
		{"key-value secret", "GITHUB_TOKEN=abc123 password: hunter2 api_key = \"k-9\"", "GITHUB_TOKEN=<redacted> password: <redacted> api_key = <redacted>"},
		{
			"secret key names with a suffix",
			"AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG aws_secret_access_key = wJalr SECRET_KEY_BASE=abcdef",
			"AWS_SECRET_ACCESS_KEY=<redacted> aws_secret_access_key = <redacted> SECRET_KEY_BASE=<redacted>",
		},
		{"quoted key-value secret", `{"password": "hunter2", "api_key":"k9secret"}`, `{"password": <redacted>, "api_key":<redacted>}`},
		{"flag secret", "gh auth login --with-token hunter2 && curl --password hunter3 -u x", "gh auth login --with-token <redacted> && curl --password <redacted> -u x"},
		{
			"flag secret names with a suffix",
			"tool --aws-secret-access-key wJalrXUt --private-key hunter2 --secret-key abcdef",
			"tool --aws-secret-access-key <redacted> --private-key <redacted> --secret-key <redacted>",
		},
		{"quoted secret with spaces", `password: "correct horse battery" --password 'a b c'`, `password: <redacted> --password <redacted>`},
		{"email with the local part git", "author git@acme.example wrote it", "author <redacted> wrote it"},
		{"URL credentials", "state_dsn: postgres://harness:s3cret@db.local:5432/runs", "state_dsn: postgres://<redacted>@db.local:5432/runs"},
		{"URL password with no user", "redis://:hunter2@db:6379", "redis://<redacted>@db:6379"},
		{"JWT", "cookie eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2lnbmF0dXJl end", "cookie <redacted> end"},
		{
			"private key block",
			"-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXk=\n-----END OPENSSH PRIVATE KEY-----\nnext",
			"<redacted>\nnext",
		},
		{
			"clean text stays unchanged",
			"git@github.com:90p-labs/harness.git token := next() if a == b /usr/local/bin/harness run status",
			"git@github.com:90p-labs/harness.git token := next() if a == b /usr/local/bin/harness run status",
		},
		{
			"a key with its value on the next line and prose stay unchanged",
			"oauth_token:\n  value: x\nsecret:\n  name: GH_TOKEN\nthe bearer token is missing\nAuthorization: Bearer\nexit status 1",
			"oauth_token:\n  value: x\nsecret:\n  name: GH_TOKEN\nthe bearer token is missing\nAuthorization: Bearer\nexit status 1",
		},
		{"token counts stay unchanged", `"input_tokens": 1234, max_tokens=4096 --tokens 5`, `"input_tokens": 1234, max_tokens=4096 --tokens 5`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.Redact(tc.in, userHome); got != tc.want {
				t.Fatalf("Redact(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRedactHomeForms(t *testing.T) {
	cases := []struct {
		name, home, in, want string
	}{
		{"encoded home with a dot", "/Users/jorge.arco", "cd /private/tmp/claude-501/-Users-jorge-arco-work-app/x", "cd /private/tmp/claude-501/~-work-app/x"},
		{"home with a trailing slash", "/srv/dev/", "cd /srv/dev/work && ls -srv-dev-work", "cd ~/work && ls ~-work"},
		{
			"short home inside other words stays",
			"/root",
			"harness verify --project-root . && ls ca-root.pem /var/lib/root/x /rootfs; cd /root/app && ls /root -root-app",
			"harness verify --project-root . && ls ca-root.pem /var/lib/root/x /rootfs; cd ~/app && ls ~ ~-app",
		},
		{
			"short home extended by a dash, a dot or a letter stays",
			"/root",
			"ls /root-backup /root.bak /rootfs -rootfs -root.pem --root",
			"ls /root-backup /root.bak /rootfs -rootfs -root.pem --root",
		},
		{"home at the end of a sentence", "/srv/dev", "cannot read /srv/dev. See -srv-dev.", "cannot read ~. See ~."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.Redact(tc.in, tc.home); got != tc.want {
				t.Fatalf("Redact(%q, %q)\n got %q\nwant %q", tc.in, tc.home, got, tc.want)
			}
		})
	}
}
