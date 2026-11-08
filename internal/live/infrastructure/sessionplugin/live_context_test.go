package sessionplugin_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jorge07/harness/internal/live/infrastructure/sessionplugin"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

var (
	errState  = errors.New("state does not open")
	errGitHub = errors.New("gh is down")
)

func TestLiveSessionContext(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		err   error
		want  shareddomain.Verdict
	}{
		{
			name:  "one line for each live run",
			lines: []string{"r1 observing pending", "r2 releasing deployed"},
			want:  shareddomain.Verdict{Kind: shareddomain.VerdictContext, Context: "live: r1 observing pending\nlive: r2 releasing deployed"},
		},
		{
			name: "a check error gives a line that names it",
			err:  errState,
			want: shareddomain.Verdict{Kind: shareddomain.VerdictContext, Context: "live: check failed: state does not open"},
		},
		{
			name: "each line of a joined error gets the prefix",
			err:  errors.Join(errState, errGitHub),
			want: shareddomain.Verdict{Kind: shareddomain.VerdictContext, Context: "live: check failed: state does not open\nlive: gh is down"},
		},
		{
			name:  "the run lines name the errors of a partial pass once",
			lines: []string{"r1 observing pending", "r2 releasing error: gh is down"},
			err:   errGitHub,
			want:  shareddomain.Verdict{Kind: shareddomain.VerdictContext, Context: "live: r1 observing pending\nlive: r2 releasing error: gh is down"},
		},
		{name: "no line and no error is an allow", want: shareddomain.Verdict{Kind: shareddomain.VerdictAllow}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plugin := sessionplugin.New(func(context.Context) ([]string, error) { return tc.lines, tc.err })

			got, err := plugin.Handle(context.Background(), shareddomain.Envelope{Event: shareddomain.EventSessionStarted})

			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("verdict = %+v, want %+v", got, tc.want)
			}
		})
	}

	t.Run("the check runs with a deadline", func(t *testing.T) {
		start := time.Now()
		var deadline time.Time
		var bounded bool
		plugin := sessionplugin.New(func(ctx context.Context) ([]string, error) {
			deadline, bounded = ctx.Deadline()
			return nil, nil
		})

		if _, err := plugin.Handle(context.Background(), shareddomain.Envelope{Event: shareddomain.EventSessionStarted}); err != nil {
			t.Fatal(err)
		}

		if !bounded || deadline.After(start.Add(sessionplugin.CheckTimeout).Add(time.Second)) {
			t.Fatalf("deadline = %v, bounded = %v, want at most %v after the start", deadline, bounded, sessionplugin.CheckTimeout)
		}
	})

	subscribes := sessionplugin.New(nil).Subscribes()
	if len(subscribes) != 1 || subscribes[0] != shareddomain.EventSessionStarted {
		t.Fatalf("subscribes = %v, want session.started", subscribes)
	}
}
