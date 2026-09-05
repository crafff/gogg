//go:build integration

package user

import (
	"context"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/crafff/gogg/apps/api/internal/auth/provider"
	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

type concurrentLoginProvider struct {
	subject string
	entered atomic.Int32
	release chan struct{}
}

type immediateLoginProvider struct {
	subject  string
	username string
}

func (*immediateLoginProvider) Name() string { return "google" }

func (*immediateLoginProvider) AuthCodeURL(state, _ string) string {
	return "https://accounts.example/authorize?state=" + url.QueryEscape(state)
}

func (p *immediateLoginProvider) Exchange(context.Context, string, string) (provider.UserInfo, error) {
	return provider.UserInfo{Subject: p.subject, Username: p.username}, nil
}

func (*concurrentLoginProvider) Name() string { return "google" }

func (*concurrentLoginProvider) AuthCodeURL(state, _ string) string {
	return "https://accounts.example/authorize?state=" + url.QueryEscape(state)
}

func (p *concurrentLoginProvider) Exchange(ctx context.Context, _, _ string) (provider.UserInfo, error) {
	if p.entered.Add(1) == 2 {
		close(p.release)
	}
	select {
	case <-p.release:
		return provider.UserInfo{Subject: p.subject, Email: "integration@example.com", Username: "Integration User"}, nil
	case <-ctx.Done():
		return provider.UserInfo{}, ctx.Err()
	}
}

func TestConcurrentFirstLoginCreatesOneUserAndTwoSessions(t *testing.T) {
	if os.Getenv("GOGG_INTTEST") == "" {
		t.Skip("set GOGG_INTTEST=1 to run PostgreSQL integration tests")
	}
	dsn := os.Getenv("GOGG_TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = "postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable"
	}
	pool, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	marker := uuid.NewString()
	returnTo := "/integration-auth/" + marker
	provider := &concurrentLoginProvider{subject: "integration-" + marker, release: make(chan struct{})}
	svc := New(pool, sqlcgen.New(pool), time.Hour, provider)

	bindings := []string{"binding-one-" + marker, "binding-two-" + marker}
	states := make([]string, len(bindings))
	for i, binding := range bindings {
		start, beginErr := svc.BeginOAuth(t.Context(), "google", returnTo, binding)
		if beginErr != nil {
			t.Fatalf("BeginOAuth(%d) error = %v", i, beginErr)
		}
		parsed, parseErr := url.Parse(start.URL)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		states[i] = parsed.Query().Get("state")
		if states[i] == "" {
			t.Fatalf("BeginOAuth(%d) returned no state", i)
		}
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id IN (
			SELECT user_id FROM user_oauth_identities WHERE provider='google' AND provider_user_id=$1
		)`, provider.subject)
		_, _ = pool.Exec(context.Background(), `DELETE FROM oauth_login_attempts WHERE return_to=$1`, returnTo)
	})

	errCh := make(chan error, len(states))
	var wg sync.WaitGroup
	for i := range states {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, gotReturnTo, completeErr := svc.CompleteOAuth(
				context.Background(), "google", "code-"+states[i], states[i], bindings[i],
				"integration-test", netip.MustParseAddr("127.0.0.1"),
			)
			if completeErr == nil && gotReturnTo != returnTo {
				completeErr = &returnToMismatch{got: gotReturnTo, want: returnTo}
			}
			errCh <- completeErr
		}(i)
	}
	wg.Wait()
	close(errCh)
	for completeErr := range errCh {
		if completeErr != nil {
			t.Fatalf("CompleteOAuth() error = %v", completeErr)
		}
	}

	var userCount, sessionCount int
	err = pool.QueryRow(t.Context(), `
		SELECT count(DISTINCT i.user_id), count(s.id)
		FROM user_oauth_identities i
		LEFT JOIN user_sessions s ON s.user_id=i.user_id
		WHERE i.provider='google' AND i.provider_user_id=$1`, provider.subject,
	).Scan(&userCount, &sessionCount)
	if err != nil {
		t.Fatal(err)
	}
	if userCount != 1 || sessionCount != 2 {
		t.Fatalf("users=%d sessions=%d, want 1 user and 2 sessions", userCount, sessionCount)
	}
}

func TestLoginTransactionRollsBackUserAndIdentityWhenSessionInsertFails(t *testing.T) {
	if os.Getenv("GOGG_INTTEST") == "" {
		t.Skip("set GOGG_INTTEST=1 to run PostgreSQL integration tests")
	}
	dsn := os.Getenv("GOGG_TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = "postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable"
	}
	pool, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	functionName := pgx.Identifier{"test_auth_session_fail_fn_" + suffix}.Sanitize()
	triggerName := pgx.Identifier{"test_auth_session_fail_tr_" + suffix}.Sanitize()
	marker := "rollback-user-agent-" + suffix
	createFunction := fmt.Sprintf(`
		CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.user_agent = %s THEN
				RAISE EXCEPTION 'intentional auth integration failure';
			END IF;
			RETURN NEW;
		END $$`, functionName, quoteLiteral(marker))
	if _, err := pool.Exec(t.Context(), createFunction); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), fmt.Sprintf(
		"CREATE TRIGGER %s BEFORE INSERT ON user_sessions FOR EACH ROW EXECUTE FUNCTION %s()",
		triggerName, functionName,
	)); err != nil {
		_, _ = pool.Exec(context.Background(), "DROP FUNCTION IF EXISTS "+functionName+"()")
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP TRIGGER IF EXISTS "+triggerName+" ON user_sessions")
		_, _ = pool.Exec(context.Background(), "DROP FUNCTION IF EXISTS "+functionName+"()")
	})

	returnTo := "/integration-auth-rollback/" + suffix
	loginProvider := &immediateLoginProvider{subject: "rollback-" + suffix, username: "Rollback User " + suffix}
	svc := New(pool, sqlcgen.New(pool), time.Hour, loginProvider)
	start, err := svc.BeginOAuth(t.Context(), "google", returnTo, "binding-"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(start.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = svc.CompleteOAuth(
		t.Context(), "google", "code", parsed.Query().Get("state"), "binding-"+suffix,
		marker, netip.MustParseAddr("127.0.0.1"),
	)
	if err == nil {
		t.Fatal("CompleteOAuth() unexpectedly succeeded")
	}

	var identityCount int
	if err := pool.QueryRow(t.Context(), `
		SELECT count(*) FROM user_oauth_identities
		WHERE provider='google' AND provider_user_id=$1`, loginProvider.subject,
	).Scan(&identityCount); err != nil {
		t.Fatal(err)
	}
	if identityCount != 0 {
		t.Fatalf("identity count = %d, want rollback to zero", identityCount)
	}
	var orphanCount int
	if err := pool.QueryRow(t.Context(), `
		SELECT count(*) FROM users WHERE display_name=$1
		  AND id NOT IN (SELECT user_id FROM user_oauth_identities)`, loginProvider.username,
	).Scan(&orphanCount); err != nil {
		t.Fatal(err)
	}
	if orphanCount != 0 {
		t.Fatalf("orphan rollback users = %d, want zero", orphanCount)
	}
	_, _ = pool.Exec(t.Context(), `DELETE FROM oauth_login_attempts WHERE return_to=$1`, returnTo)
}

type returnToMismatch struct{ got, want string }

func (e *returnToMismatch) Error() string {
	return "returnTo mismatch: got " + e.got + ", want " + e.want
}

func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
