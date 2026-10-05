package guard

import "testing"

// The guard runs before every command an agent runs: no input may make it
// panic.
func FuzzInspect(f *testing.F) {
	for _, s := range []string{"> out.txt", "rm -rf build", "echo 'x' > a.sh && sh a.sh", "cat <<EOF\nrm -rf /\nEOF",
		`python -c "import shutil"`, "npm run", "make -f", "curl -o", ">>", "<<", "sh", "x=1", "'", `"`, "$(", "`"} {
		f.Add(s)
	}
	read := files{"a.sh": "rm -rf build\n", "package.json": `{"scripts":{"x":"rimraf y"}}`, "Makefile": "all:\n\trm -rf z\n"}.read
	f.Fuzz(func(t *testing.T, line string) {
		_ = Inspect(line, read)
	})
}
