package audit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/jefmonjor/specforge/v6/internal/domain/stack"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

const maxSourceBytes = 200_000

var sourceExt = map[string]bool{
	".go": true, ".java": true, ".kt": true, ".scala": true, ".groovy": true,
	".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".py": true, ".rb": true, ".php": true, ".cs": true, ".rs": true, ".swift": true,
	".c": true, ".cc": true, ".cpp": true, ".h": true, ".sql": true, ".sh": true,
	".yaml": true, ".yml": true, ".properties": true, ".toml": true, ".xml": true, ".tf": true,
}

var testKinds = []stack.Profile{{Kind: stack.Go}, {Kind: stack.Maven}, {Kind: stack.Node}, {Kind: stack.Python}}

// targets returns the code to audit, split in chunks that fit a prompt.
func (s *Service) targets(ctx context.Context, o Options) ([]string, string, error) {
	if o.Scope == ScopeDiff {
		base := o.Base
		if base == "" {
			var err error
			if base, err = s.d.VCS.DefaultBase(ctx, o.Root); err != nil {
				return nil, "", err
			}
		}
		diff, err := s.d.VCS.Diff(ctx, o.Root, base)
		if err != nil {
			return nil, base, err
		}
		if strings.TrimSpace(diff) == "" {
			return nil, base, nil
		}
		return splitText("```diff\n"+diff+"\n```", o.MaxChunkBytes), base, nil
	}

	files, err := s.d.VCS.Files(ctx, o.Root)
	if errors.Is(err, ports.ErrNotARepository) && s.d.Lister != nil {
		files, err = s.d.Lister(o.Root)
	}
	if err != nil {
		return nil, "", fmt.Errorf("listing source files: %w", err)
	}
	sort.Strings(files)

	var chunks []string
	var cur strings.Builder
	for _, f := range files {
		if !isAuditable(f) {
			continue
		}
		data, err := s.d.Files.ReadFile(path.Join(o.Root, f))
		if err != nil || len(data) > maxSourceBytes || bytes.IndexByte(data, 0) >= 0 {
			continue
		}
		block := "### `" + f + "`\n```\n" + string(data) + "\n```\n\n"
		if cur.Len() > 0 && cur.Len()+len(block) > o.MaxChunkBytes {
			chunks = append(chunks, cur.String())
			cur.Reset()
		}
		if len(block) > o.MaxChunkBytes {
			block = block[:o.MaxChunkBytes] + "\n[… truncated …]\n```\n"
		}
		cur.WriteString(block)
	}
	if cur.Len() > 0 {
		chunks = append(chunks, cur.String())
	}
	return chunks, "", nil
}

func isAuditable(f string) bool {
	base := path.Base(f)
	if base == "Dockerfile" {
		return true
	}
	if !sourceExt[strings.ToLower(path.Ext(f))] {
		return false
	}
	for _, seg := range strings.Split(f, "/") {
		if stack.IgnoredDirs[seg] {
			return false
		}
	}
	for _, p := range testKinds {
		if p.IsTestFile(f) {
			return false
		}
	}
	return true
}

func splitText(s string, max int) []string {
	var out []string
	for len(s) > max {
		cut := strings.LastIndex(s[:max], "\ndiff --git ")
		if cut <= 0 {
			cut = max
		}
		out = append(out, s[:cut])
		s = s[cut:]
	}
	if strings.TrimSpace(s) != "" {
		out = append(out, s)
	}
	return out
}
