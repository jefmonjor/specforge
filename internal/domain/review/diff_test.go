package review

import (
	"reflect"
	"testing"
)

const sampleDiff = `diff --git a/internal/pay/net.go b/internal/pay/net.go
index 1111111..2222222 100644
--- a/internal/pay/net.go
+++ b/internal/pay/net.go
@@ -10,7 +10,8 @@ func Net(gross Money) Money {
 	tax := gross.Times(rate)
-	return gross.Minus(tax)
+	net := gross.Minus(tax)
+	return net.Plus(bonus)
 }
 
 func rate() Rate {
@@ -40,3 +41,0 @@ func old() {
-	legacy()
-	legacy()
-	legacy()
diff --git a/internal/pay/bonus.go b/internal/pay/bonus.go
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/internal/pay/bonus.go
@@ -0,0 +1,3 @@
+package pay
+
+var bonus = Money(5)
diff --git a/logo.png b/logo.png
Binary files a/logo.png and b/logo.png differ
diff --git a/old.go b/old.go
deleted file mode 100644
--- a/old.go
+++ /dev/null
@@ -1,2 +0,0 @@
-package pay
-func gone() {}
`

func TestParseDiff(t *testing.T) {
	d, err := ParseDiff(sampleDiff)
	if err != nil {
		t.Fatal(err)
	}
	want := []Hunk{
		{Path: "internal/pay/net.go", Start: 11, End: 13},
		{Path: "internal/pay/net.go", Start: 42, End: 42},
		{Path: "internal/pay/bonus.go", Start: 1, End: 4},
	}
	if !reflect.DeepEqual(d.Hunks, want) {
		t.Fatalf("hunks = %+v, want %+v", d.Hunks, want)
	}
	if !reflect.DeepEqual(d.New, []string{"internal/pay/bonus.go"}) || len(d.Files) != 2 {
		t.Fatalf("files = %v new = %v", d.Files, d.New)
	}
	for _, c := range []struct {
		path string
		line int
		want bool
	}{
		{"internal/pay/net.go", 10, false}, // context
		{"internal/pay/net.go", 11, true},
		{"internal/pay/net.go", 12, true},
		{"internal/pay/net.go", 13, false},
		{"internal/pay/net.go", 41, true}, // around the deletion
		{"internal/pay/net.go", 43, true},
		{"internal/pay/net.go", 44, false},
		{"internal/pay/bonus.go", 3, true},
		{"internal/pay/other.go", 11, false},
	} {
		if got := d.Changed(c.path, c.line); got != c.want {
			t.Errorf("Changed(%s:%d) = %v, want %v", c.path, c.line, got, c.want)
		}
	}
}

func TestParseDiffZeroContext(t *testing.T) {
	d, err := ParseDiff("diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -3 +3 @@\n-x := 1\n+x := 2\n@@ -9,0 +10,2 @@\n+y()\n+z()\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := []Hunk{{Path: "a.go", Start: 3, End: 4}, {Path: "a.go", Start: 10, End: 12}}; !reflect.DeepEqual(d.Hunks, want) {
		t.Fatalf("hunks = %+v", d.Hunks)
	}
}

func TestParseDiffRejectsAMalformedHunk(t *testing.T) {
	if _, err := ParseDiff("diff --git a/a b/a\n--- a/a\n+++ b/a\n@@ nonsense @@\n"); err == nil {
		t.Fatal("a malformed hunk header is an error")
	}
}

func TestNewFileDiffRoundTrips(t *testing.T) {
	d, err := ParseDiff(NewFileDiff("pay/bonus.go", []byte("package pay\n\nvar bonus = 5\n")))
	if err != nil || !d.IsNew("pay/bonus.go") || !d.Changed("pay/bonus.go", 3) || d.Changed("pay/bonus.go", 4) {
		t.Fatalf("diff = %+v %v", d, err)
	}
	if d, _ := ParseDiff(NewFileDiff("logo.png", []byte{1, 0, 2})); len(d.Hunks) != 0 || !d.IsNew("logo.png") {
		t.Fatalf("binary = %+v", d)
	}
}
