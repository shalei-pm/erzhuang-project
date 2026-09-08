package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// These tests cover credential consumption, not whether a newly issued JWT
// proves interactive SSO reauthentication. That remains an upstream contract.
func TestMySQLAuthSessionLifetimeSQL(t *testing.T) {
	for name, query := range map[string]string{
		"touch":    mysqlAuthSessionTouchSQL,
		"fallback": mysqlAuthSessionValidSQL,
	} {
		t.Run(name, func(t *testing.T) {
			for _, want := range []string{
				"expires_at > utc_timestamp(3)",
				"last_activity_at > date_sub(utc_timestamp(3), interval 30 minute)",
				"created_at > date_sub(utc_timestamp(3), interval 8 hour)",
				"revoked_at is null",
			} {
				if !strings.Contains(query, want) {
					t.Errorf("%s query missing %q", name, want)
				}
			}
		})
	}
	query := compactSessionSQL(mysqlAuthSessionTouchSQL)
	want := "expires_at = least(date_add(created_at, interval 8 hour), date_add(greatest(last_activity_at, utc_timestamp(3)), interval 30 minute))"
	if !strings.Contains(query, want) {
		t.Errorf("touch must derive expiry from activity and cap it at creation + 8h: %s", query)
	}
}

func TestMySQLAuthSessionFingerprintCreate(t *testing.T) {
	fingerprint := "jwt-sha256:v1:" + strings.Repeat("a", 64)
	db := newSessionScriptDB(t,
		sessionSQLStep{op: "begin"},
		sessionSQLStep{op: "query", contains: "from tb_users", wants: []string{"id = ?", "enabled = 1", "for update"}, args: []any{int64(7)}, rows: []driver.Value{int64(7)}},
		sessionSQLStep{op: "query", contains: "from tb_auth_sessions", wants: []string{"user_id = ?", "sso_subject = ?", "for update"}, rejects: []string{"expires_at", "revoked_at", "created_at"}, args: []any{int64(7), fingerprint}},
		sessionSQLStep{op: "exec", contains: "insert into tb_auth_sessions", affected: 1},
		sessionSQLStep{op: "commit"},
	)
	token, err := NewMySQLStore(db).CreateAuthSession(context.Background(), AuthSessionCreate{UserID: 7, SSOSubject: fingerprint})
	if err != nil || token == "" {
		t.Fatalf("new fingerprint must commit a session: token_present=%t err=%v", token != "", err)
	}
}

func TestMySQLAuthSessionFingerprintReplay(t *testing.T) {
	for _, state := range []string{"active", "idle-expired", "absolute-expired", "revoked"} {
		t.Run(state, func(t *testing.T) {
			db := newSessionScriptDB(t,
				sessionSQLStep{op: "begin"},
				sessionSQLStep{op: "query", contains: "from tb_users", rows: []driver.Value{int64(7)}},
				sessionSQLStep{op: "query", contains: "from tb_auth_sessions", rejects: []string{"expires_at", "revoked_at", "created_at"}, rows: []driver.Value{int64(19)}},
				sessionSQLStep{op: "rollback"},
			)
			token, err := NewMySQLStore(db).CreateAuthSession(context.Background(), AuthSessionCreate{
				UserID: 7, SSOSubject: "jwt-sha256:v1:" + strings.Repeat("b", 64),
			})
			if !errors.Is(err, errSessionReauthenticationRequired) || token != "" {
				t.Fatalf("consumed fingerprint must reject creation regardless of session state: token_present=%t err=%v", token != "", err)
			}
		})
	}
}

func TestMySQLAuthSessionAbsoluteExpiryResult(t *testing.T) {
	db := newSessionScriptDB(t,
		sessionSQLStep{op: "begin"},
		sessionSQLStep{op: "exec", contains: "update tb_auth_sessions"},
		sessionSQLStep{op: "query", contains: "select 1", wants: []string{"created_at > date_sub(utc_timestamp(3), interval 8 hour)"}},
		sessionSQLStep{op: "query", contains: "select 1", wants: []string{"created_at <= date_sub(utc_timestamp(3), interval 8 hour)", "session_token_hash = ?", "user_id = ?", "revoked_at is null", "for update"}, rows: []driver.Value{int64(1)}},
		sessionSQLStep{op: "rollback"},
	)
	ok, err := NewMySQLStore(db).TouchAuthSession(context.Background(), "token", 7, time.Now(), 24*time.Hour)
	if ok || !errors.Is(err, errSessionAbsoluteTimeout) {
		t.Fatalf("absolute expiry must return false and a classified error: ok=%t err=%v", ok, err)
	}
}

func TestMySQLAuthSessionTouchCommitFailure(t *testing.T) {
	failure := errors.New("commit failed")
	for _, affected := range []int64{0, 1} {
		t.Run(fmt.Sprint(affected), func(t *testing.T) {
			steps := []sessionSQLStep{{op: "begin"}, {op: "exec", contains: "update tb_auth_sessions", affected: affected}}
			if affected == 0 {
				steps = append(steps, sessionSQLStep{op: "query", contains: "select 1", rows: []driver.Value{int64(1)}})
			}
			steps = append(steps, sessionSQLStep{op: "commit", err: failure})
			db := newSessionScriptDB(t, steps...)
			ok, err := NewMySQLStore(db).TouchAuthSession(context.Background(), "token", 7, time.Now(), time.Minute)
			if ok || !errors.Is(err, failure) {
				t.Fatalf("failed commit must not report active: ok=%t err=%v", ok, err)
			}
		})
	}
}

func TestMySQLAuthSessionFingerprintCreateFailures(t *testing.T) {
	failure := errors.New("database failure")
	for _, stage := range []string{"begin", "user missing", "user query", "consumed query", "insert", "commit"} {
		t.Run(stage, func(t *testing.T) {
			steps := []sessionSQLStep{
				{op: "begin"},
				{op: "query", contains: "from tb_users", rows: []driver.Value{int64(7)}},
				{op: "query", contains: "from tb_auth_sessions"},
				{op: "exec", contains: "insert into tb_auth_sessions", affected: 1},
				{op: "commit"},
			}
			index := map[string]int{"begin": 0, "user missing": 1, "user query": 1, "consumed query": 2, "insert": 3, "commit": 4}[stage]
			wantErr := failure
			if stage == "user missing" {
				steps[index].rows = nil
				wantErr = errSessionReauthenticationRequired
			} else {
				steps[index].err = failure
			}
			steps = steps[:index+1]
			if index > 0 && stage != "commit" {
				steps = append(steps, sessionSQLStep{op: "rollback"})
			}
			db := newSessionScriptDB(t, steps...)
			token, err := NewMySQLStore(db).CreateAuthSession(context.Background(), AuthSessionCreate{
				UserID: 7, SSOSubject: "jwt-sha256:v1:" + strings.Repeat("c", 64),
			})
			if token != "" || !errors.Is(err, wantErr) {
				t.Fatalf("failed creation must not expose a token: token_present=%t err=%v", token != "", err)
			}
		})
	}
}

func TestMySQLAuthSessionLegacySubjectCompatibility(t *testing.T) {
	for _, subject := range []string{"", "ordinary-subject"} {
		t.Run(subject, func(t *testing.T) {
			db := newSessionScriptDB(t, sessionSQLStep{op: "exec", contains: "insert into tb_auth_sessions", affected: 1})
			if token, err := NewMySQLStore(db).CreateAuthSession(context.Background(), AuthSessionCreate{UserID: 7, SSOSubject: subject}); err != nil || token == "" {
				t.Fatalf("legacy direct store caller must remain supported: token_present=%t err=%v", token != "", err)
			}
		})
	}
}

func TestMySQLAuthSessionTouchFailures(t *testing.T) {
	failure := errors.New("database failure")
	for index, stage := range []string{"begin", "update", "valid lookup", "absolute lookup"} {
		t.Run(stage, func(t *testing.T) {
			steps := []sessionSQLStep{
				{op: "begin"},
				{op: "exec", contains: "update tb_auth_sessions"},
				{op: "query", contains: "select 1"},
				{op: "query", contains: "select 1"},
			}
			steps[index].err = failure
			steps = steps[:index+1]
			if index > 0 {
				steps = append(steps, sessionSQLStep{op: "rollback"})
			}
			db := newSessionScriptDB(t, steps...)
			ok, err := NewMySQLStore(db).TouchAuthSession(context.Background(), "token", 7, time.Now(), time.Minute)
			if ok || !errors.Is(err, failure) {
				t.Fatalf("database failure must not authenticate: ok=%t err=%v", ok, err)
			}
		})
	}
}

func TestMySQLAuthSessionInactiveResult(t *testing.T) {
	db := newSessionScriptDB(t,
		sessionSQLStep{op: "begin"},
		sessionSQLStep{op: "exec", contains: "update tb_auth_sessions"},
		sessionSQLStep{op: "query", contains: "select 1"},
		sessionSQLStep{op: "query", contains: "select 1", wants: []string{"revoked_at is null", "user_id = ?"}},
		sessionSQLStep{op: "rollback"},
	)
	ok, err := NewMySQLStore(db).TouchAuthSession(context.Background(), "token", 7, time.Now(), time.Minute)
	if ok || err != nil {
		t.Fatalf("inactive non-absolute session must return false,nil: ok=%t err=%v", ok, err)
	}
}

func TestMySQLAuthSessionStatusSQLContract(t *testing.T) {
	query := compactSessionSQL(mysqlAuthSessionStatusSQL)
	if !strings.HasPrefix(query, "select ") || strings.Count(query, "?") != 2 {
		t.Fatalf("status must be a single SELECT bound only to hash and user: %s", query)
	}
	for _, forbidden := range []string{"for update", ";", "insert ", "update ", "delete ", "set "} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("status query must not lock or mutate: found %q", forbidden)
		}
	}
}

func TestMySQLAuthSessionStatus(t *testing.T) {
	failure := errors.New("status query failed")
	for _, tc := range []struct {
		name     string
		row      []driver.Value
		queryErr error
		wantErr  error
	}{
		{name: "active", row: []driver.Value{int64(1800000), int64(28800000), false}},
		{name: "last millisecond", row: []driver.Value{int64(1), int64(1), false}},
		{name: "missing", wantErr: errSessionIdleTimeout},
		{name: "revoked", row: []driver.Value{int64(1800000), int64(28800000), true}, wantErr: errSessionIdleTimeout},
		{name: "idle exactly", row: []driver.Value{int64(0), int64(1000), false}, wantErr: errSessionIdleTimeout},
		{name: "idle after", row: []driver.Value{int64(-1), int64(1000), false}, wantErr: errSessionIdleTimeout},
		{name: "absolute exactly", row: []driver.Value{int64(1000), int64(0), false}, wantErr: errSessionAbsoluteTimeout},
		{name: "absolute after", row: []driver.Value{int64(1000), int64(-1), false}, wantErr: errSessionAbsoluteTimeout},
		{name: "absolute before idle", row: []driver.Value{int64(0), int64(0), false}, wantErr: errSessionAbsoluteTimeout},
		{name: "absolute before revoked", row: []driver.Value{int64(1000), int64(0), true}, wantErr: errSessionAbsoluteTimeout},
		{name: "query failure", queryErr: failure, wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := "local-session-token"
			hash := hashAuthSessionToken(token)
			db := newSessionScriptDB(t, sessionSQLStep{
				op: "query", contains: "from tb_auth_sessions",
				wants: []string{
					"timestampdiff(microsecond, utc_timestamp(3), least(expires_at, date_add(last_activity_at, interval 30 minute))) div 1000",
					"timestampdiff(microsecond, utc_timestamp(3), date_add(created_at, interval 8 hour)) div 1000",
					"revoked_at is not null", "session_token_hash = ?", "user_id = ?",
				},
				rejects: []string{"for update", "insert ", "update ", "delete ", "set ", "date_add(?,", token},
				args:    []any{fmt.Sprintf("%x", hash), int64(7)}, row: tc.row, err: tc.queryErr,
			})
			reader, ok := any(NewMySQLStore(db)).(interface {
				GetAuthSessionStatus(context.Context, string, int64, time.Time) (AuthSessionStatus, error)
			})
			if !ok {
				t.Fatal("MySQLStore must implement the non-renewing session status reader")
			}
			status, err := reader.GetAuthSessionStatus(context.Background(), token, 7, time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("status error=%v, want=%v", err, tc.wantErr)
			}
			if err != nil {
				if status != (AuthSessionStatus{}) {
					t.Fatal("failed status lookup must return zero status")
				}
				return
			}
			if status.IdleRemainingMS != tc.row[0].(int64) || status.AbsoluteRemainingMS != tc.row[1].(int64) {
				t.Fatalf("remaining time must come from the database: %+v", status)
			}
		})
	}
}

// The database must be explicitly declared and dedicated to this test suite.
// This helper never creates or migrates tables and only removes its own rows.
func openIsolatedMySQLSessionDB(t *testing.T) (*sql.DB, int64) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	declared := strings.TrimSpace(os.Getenv("MYSQL_AUTH_SESSION_TEST_DATABASE"))
	if dsn == "" || declared == "" {
		t.Skip("MYSQL_TEST_DSN and MYSQL_AUTH_SESSION_TEST_DATABASE are required for isolated session integration tests")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil || !isSafeMySQLTestDSN(dsn) || cfg.DBName != declared || !strings.HasSuffix(strings.ToLower(declared), "_auth_session_test") {
		t.Fatal("session integration tests require an explicitly declared isolated *_auth_session_test database")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("open isolated session test database failed")
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var actual string
	if err := db.QueryRowContext(ctx, "select database()").Scan(&actual); err != nil || actual != declared {
		t.Fatal("isolated session test database identity check failed")
	}
	unique, err := newAuthSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	result, err := db.ExecContext(ctx, "insert into tb_users (email, enabled) values (?, 1)", "session-test-"+unique+"@example.invalid")
	if err != nil {
		t.Fatal("create isolated session test user failed")
	}
	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := db.ExecContext(ctx, "delete from tb_auth_sessions where user_id = ?", userID); err != nil {
			t.Error("cleanup test-owned sessions failed")
		}
		if _, err := db.ExecContext(ctx, "delete from tb_users where id = ?", userID); err != nil {
			t.Error("cleanup test-owned user failed")
		}
	})
	return db, userID
}

func TestMySQLAuthSessionLifetimeIntegration(t *testing.T) {
	db, userID := openIsolatedMySQLSessionDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// A single connection and a fixed MySQL session clock make exact boundaries
	// deterministic without allowing application callers to control SQL time.
	fixed := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	if _, err := db.ExecContext(ctx, "set timestamp = ?", fixed.Unix()); err != nil {
		t.Fatal(err)
	}
	var databaseNow string
	if err := db.QueryRowContext(ctx, "select date_format(utc_timestamp(3), '%Y-%m-%d %H:%i:%s.%f')").Scan(&databaseNow); err != nil || databaseNow != "2026-09-07 00:00:00.000000" {
		t.Fatalf("fixed database clock unavailable: now=%q err=%v", databaseNow, err)
	}
	store := NewMySQLStore(db)
	for _, tc := range []struct {
		name         string
		age          time.Duration
		idle         time.Duration
		expiry       time.Duration
		revoked      bool
		wrongUser    bool
		wantActive   bool
		wantAbsolute bool
	}{
		{name: "idle before 1ms", age: time.Hour, idle: 30*time.Minute - time.Millisecond, expiry: time.Hour, wantActive: true},
		{name: "idle exactly", age: time.Hour, idle: 30 * time.Minute, expiry: time.Hour},
		{name: "idle after 1ms", age: time.Hour, idle: 30*time.Minute + time.Millisecond, expiry: time.Hour},
		{name: "absolute before 1ms", age: 8*time.Hour - time.Millisecond, idle: time.Minute, expiry: time.Hour, wantActive: true},
		{name: "absolute exactly", age: 8 * time.Hour, idle: time.Minute, expiry: time.Hour, wantAbsolute: true},
		{name: "absolute after 1ms", age: 8*time.Hour + time.Millisecond, idle: time.Minute, expiry: time.Hour, wantAbsolute: true},
		{name: "both deadlines", age: 9 * time.Hour, idle: time.Hour, expiry: -time.Minute, wantAbsolute: true},
		{name: "cap at 8h", age: 7*time.Hour + 50*time.Minute, idle: time.Minute, expiry: time.Hour, wantActive: true},
		{name: "expiry exactly", age: time.Hour, idle: time.Minute},
		{name: "revoked absolute", age: 9 * time.Hour, idle: time.Minute, expiry: time.Hour, revoked: true},
		{name: "wrong user absolute", age: 9 * time.Hour, idle: time.Minute, expiry: time.Hour, wrongUser: true},
		{name: "same millisecond", expiry: 30 * time.Minute, wantActive: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token, err := store.CreateAuthSession(ctx, AuthSessionCreate{UserID: userID})
			if err != nil {
				t.Fatal(err)
			}
			hash := hashAuthSessionToken(token)
			hashText := fmt.Sprintf("%x", hash)
			_, err = db.ExecContext(ctx, `update tb_auth_sessions
				set created_at = date_sub(utc_timestamp(3), interval ? microsecond),
				last_activity_at = date_sub(utc_timestamp(3), interval ? microsecond),
				expires_at = date_add(utc_timestamp(3), interval ? microsecond),
				revoked_at = if(?, utc_timestamp(3), null)
				where session_token_hash = ? and user_id = ?`, tc.age.Microseconds(), tc.idle.Microseconds(), tc.expiry.Microseconds(), tc.revoked, hashText, userID)
			if err != nil {
				t.Fatal(err)
			}
			snapshot := func() string {
				var value string
				err := db.QueryRowContext(ctx, `select concat_ws('|', created_at, last_activity_at, expires_at, coalesce(revoked_at, 'NULL'))
					from tb_auth_sessions where session_token_hash = ?`, hashText).Scan(&value)
				if err != nil {
					t.Fatal(err)
				}
				return value
			}
			before := snapshot()
			requestUserID := userID
			if tc.wrongUser {
				requestUserID = -1
			}
			var wantStatusErr error
			switch {
			case tc.wrongUser:
				wantStatusErr = errSessionIdleTimeout
			case tc.age >= 8*time.Hour:
				wantStatusErr = errSessionAbsoluteTimeout
			case tc.revoked || tc.idle >= 30*time.Minute || tc.expiry <= 0:
				wantStatusErr = errSessionIdleTimeout
			}
			status, err := store.GetAuthSessionStatus(ctx, token, requestUserID, fixed.Add(24*time.Hour))
			if !errors.Is(err, wantStatusErr) {
				t.Fatalf("status error=%v, want=%v", err, wantStatusErr)
			}
			if err == nil {
				idleRemaining := min(tc.expiry, 30*time.Minute-tc.idle)
				if status.IdleRemainingMS != idleRemaining.Milliseconds() || status.AbsoluteRemainingMS != (8*time.Hour-tc.age).Milliseconds() {
					t.Fatalf("incorrect database-clock status: %+v", status)
				}
			} else if status != (AuthSessionStatus{}) {
				t.Fatal("failed status lookup returned nonzero remaining time")
			}
			if after := snapshot(); after != before {
				t.Fatal("read-only status query modified the session")
			}
			active, err := store.TouchAuthSession(ctx, token, requestUserID, fixed.Add(24*time.Hour), 24*time.Hour)
			if active != tc.wantActive || (tc.wantAbsolute && !errors.Is(err, errSessionAbsoluteTimeout)) || (!tc.wantAbsolute && err != nil) {
				t.Fatalf("touch result: active=%t err=%v; want active=%t absolute=%t", active, err, tc.wantActive, tc.wantAbsolute)
			}
			if !active {
				if after := snapshot(); after != before {
					t.Fatal("rejected touch modified the session")
				}
				return
			}
			var unchangedCreation, exactExpiry, currentActivity bool
			err = db.QueryRowContext(ctx, `select
				created_at = date_sub(utc_timestamp(3), interval ? microsecond),
				expires_at = least(date_add(created_at, interval 8 hour), date_add(utc_timestamp(3), interval 30 minute)),
				last_activity_at = utc_timestamp(3)
				from tb_auth_sessions where session_token_hash = ?`, tc.age.Microseconds(), hashText).Scan(&unchangedCreation, &exactExpiry, &currentActivity)
			if err != nil || !unchangedCreation || !exactExpiry || !currentActivity {
				t.Fatalf("invalid renewed session: creation=%t expiry=%t activity=%t err=%v", unchangedCreation, exactExpiry, currentActivity, err)
			}
		})
	}
}

func TestMySQLAuthSessionStatusDoesNotRenewIntegration(t *testing.T) {
	db, userID := openIsolatedMySQLSessionDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fixed := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	if _, err := db.ExecContext(ctx, "set timestamp = ?", fixed.Unix()); err != nil {
		t.Fatal(err)
	}
	store := NewMySQLStore(db)
	token, err := store.CreateAuthSession(ctx, AuthSessionCreate{UserID: userID})
	if err != nil {
		t.Fatal(err)
	}
	for _, elapsed := range []time.Duration{0, time.Minute, 29 * time.Minute, 30 * time.Minute} {
		if _, err := db.ExecContext(ctx, "set timestamp = ?", fixed.Add(elapsed).Unix()); err != nil {
			t.Fatal(err)
		}
		status, err := store.GetAuthSessionStatus(ctx, token, userID, fixed.Add(-24*time.Hour))
		if elapsed == 30*time.Minute {
			if !errors.Is(err, errSessionIdleTimeout) {
				t.Fatalf("status polling must not prevent idle expiry: %v", err)
			}
		} else if err != nil || status.IdleRemainingMS != (30*time.Minute-elapsed).Milliseconds() || status.AbsoluteRemainingMS != (8*time.Hour-elapsed).Milliseconds() {
			t.Fatalf("status polling changed the deadlines: status=%+v err=%v", status, err)
		}
	}
	var unchanged bool
	hash := hashAuthSessionToken(token)
	if err := db.QueryRowContext(ctx, `select created_at = last_activity_at and expires_at = date_add(created_at, interval 30 minute)
		from tb_auth_sessions where session_token_hash = ? and user_id = ?`, fmt.Sprintf("%x", hash), userID).Scan(&unchanged); err != nil || !unchanged {
		t.Fatalf("status checks must leave persisted activity and expiry unchanged: unchanged=%t err=%v", unchanged, err)
	}
}

func TestMySQLAuthSessionFingerprintConcurrencyIntegration(t *testing.T) {
	db, userID := openIsolatedMySQLSessionDB(t)
	db.SetMaxOpenConns(8)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := NewMySQLStore(db)
	fingerprint := "jwt-sha256:v1:" + strings.Repeat("d", 64)
	const workers = 8
	start := make(chan struct{})
	results := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			token, err := store.CreateAuthSession(ctx, AuthSessionCreate{UserID: userID, SSOSubject: fingerprint})
			if (err == nil) != (token != "") {
				err = errors.New("creation exposed an inconsistent token/error result")
			}
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, errSessionReauthenticationRequired) {
			t.Errorf("unexpected concurrent creation error: %v", err)
		}
	}
	var count int
	if err := db.QueryRowContext(ctx, "select count(*) from tb_auth_sessions where user_id = ? and sso_subject = ?", userID, fingerprint).Scan(&count); err != nil || count != 1 || successes != 1 {
		t.Fatalf("concurrent credential consumption: successes=%d rows=%d err=%v", successes, count, err)
	}
	for _, state := range []string{
		"last_activity_at = date_sub(utc_timestamp(3), interval 31 minute)",
		"created_at = date_sub(utc_timestamp(3), interval 9 hour)",
		"revoked_at = utc_timestamp(3)",
	} {
		if _, err := db.ExecContext(ctx, "update tb_auth_sessions set "+state+" where user_id = ? and sso_subject = ?", userID, fingerprint); err != nil {
			t.Fatal(err)
		}
		if token, err := store.CreateAuthSession(ctx, AuthSessionCreate{UserID: userID, SSOSubject: fingerprint}); token != "" || !errors.Is(err, errSessionReauthenticationRequired) {
			t.Fatalf("consumed credential was reusable: token_present=%t err=%v", token != "", err)
		}
	}
	if token, err := store.CreateAuthSession(ctx, AuthSessionCreate{UserID: userID, SSOSubject: "jwt-sha256:v1:" + strings.Repeat("e", 64)}); err != nil || token == "" {
		t.Fatalf("different credential should be eligible: token_present=%t err=%v", token != "", err)
	}
}

type sessionSQLStep struct {
	op       string
	contains string
	wants    []string
	rejects  []string
	args     []any
	rows     []driver.Value
	row      []driver.Value
	affected int64
	err      error
}

type sessionScriptDriver struct {
	t     *testing.T
	steps []sessionSQLStep
}

var sessionScriptID atomic.Uint64

func newSessionScriptDB(t *testing.T, steps ...sessionSQLStep) *sql.DB {
	t.Helper()
	d := &sessionScriptDriver{t: t, steps: steps}
	name := fmt.Sprintf("auth-session-script-%d", sessionScriptID.Add(1))
	sql.Register(name, d)
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		db.Close()
		if len(d.steps) != 0 {
			t.Errorf("%d SQL steps not executed; next=%s", len(d.steps), d.steps[0].op)
		}
	})
	return db
}

func compactSessionSQL(query string) string {
	return strings.Join(strings.Fields(query), " ")
}

func (d *sessionScriptDriver) next(op, query string, args []driver.NamedValue) (sessionSQLStep, error) {
	d.t.Helper()
	if len(d.steps) == 0 {
		d.t.Errorf("unexpected SQL operation: %s %s", op, query)
		return sessionSQLStep{}, errors.New("unexpected SQL operation")
	}
	step := d.steps[0]
	d.steps = d.steps[1:]
	query = compactSessionSQL(query)
	if step.op != op || !strings.Contains(query, step.contains) {
		d.t.Errorf("SQL operation = %s %s, want %s containing %q", op, query, step.op, step.contains)
		return step, errors.New("unexpected SQL operation")
	}
	for _, want := range step.wants {
		if !strings.Contains(query, want) {
			d.t.Errorf("SQL missing %q: %s", want, query)
		}
	}
	for _, reject := range step.rejects {
		if strings.Contains(query, reject) {
			d.t.Errorf("SQL must not contain %q: %s", reject, query)
		}
	}
	if step.args != nil {
		values := make([]any, len(args))
		for i, arg := range args {
			values[i] = arg.Value
		}
		if !reflect.DeepEqual(values, step.args) {
			d.t.Errorf("unexpected SQL bindings for %s", op)
		}
	}
	return step, step.err
}

func (d *sessionScriptDriver) Open(string) (driver.Conn, error) {
	return &sessionScriptConn{d}, nil
}

type sessionScriptConn struct{ d *sessionScriptDriver }

func (c *sessionScriptConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare unsupported")
}
func (c *sessionScriptConn) Close() error { return nil }
func (c *sessionScriptConn) Begin() (driver.Tx, error) {
	if _, err := c.d.next("begin", "", nil); err != nil {
		return nil, err
	}
	return &sessionScriptTx{c.d}, nil
}
func (c *sessionScriptConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	step, err := c.d.next("exec", query, args)
	return driver.RowsAffected(step.affected), err
}
func (c *sessionScriptConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	step, err := c.d.next("query", query, args)
	columns := []string{"value"}
	if step.row != nil {
		columns = make([]string, len(step.row))
		for i := range columns {
			columns[i] = fmt.Sprintf("value%d", i)
		}
	}
	return &sessionScriptRows{values: step.rows, row: step.row, columns: columns}, err
}

type sessionScriptTx struct{ d *sessionScriptDriver }

func (tx *sessionScriptTx) Commit() error {
	_, err := tx.d.next("commit", "", nil)
	return err
}
func (tx *sessionScriptTx) Rollback() error {
	_, err := tx.d.next("rollback", "", nil)
	return err
}

type sessionScriptRows struct {
	values  []driver.Value
	row     []driver.Value
	columns []string
}

func (r *sessionScriptRows) Columns() []string { return r.columns }
func (r *sessionScriptRows) Close() error      { return nil }
func (r *sessionScriptRows) Next(dest []driver.Value) error {
	if r.row != nil {
		copy(dest, r.row)
		r.row = nil
		return nil
	}
	if len(r.values) == 0 {
		return io.EOF
	}
	dest[0] = r.values[0]
	r.values = r.values[1:]
	return nil
}
