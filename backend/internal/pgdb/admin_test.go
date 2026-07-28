package pgdb

import (
	"io"
	"strings"
	"testing"
)

func TestFilterRestoreSQL_stripsTransactionTimeout(t *testing.T) {
	in := strings.NewReader(`SET client_encoding = 'UTF8';
SET transaction_timeout = 0;
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('transaction_timeout', '0', false);
CREATE TABLE t (id int);
`)
	out, err := io.ReadAll(filterRestoreSQL(in))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Contains(got, "transaction_timeout") {
		t.Fatalf("expected transaction_timeout stripped, got:\n%s", got)
	}
	for _, want := range []string{
		"SET client_encoding = 'UTF8';",
		"SET standard_conforming_strings = on;",
		"CREATE TABLE t (id int);",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}
