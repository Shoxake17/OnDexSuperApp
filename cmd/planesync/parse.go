package main

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Commit — `git log` dan bitta yozuv.
type Commit struct {
	SHA     string
	Subject string
	Body    string
}

var (
	issueRef = regexp.MustCompile(`ONDEXECOSY-(\d+)`)
	// "Tugadi: ONDEXECOSY-12, ONDEXECOSY-13" / "Closes ONDEXECOSY-12" /
	// "Fixes ONDEXECOSY-12" / "Yopadi ONDEXECOSY-12".
	closeRef = regexp.MustCompile(`(?i)\b(?:tugadi|yopadi|closes|fixes)\b[:\s]+((?:ONDEXECOSY-\d+[\s,;]*(?:va\s+|and\s+)?)+)`)
)

// Refs — commit eslatgan barcha vazifalar va ulardan qaysilari "yopiladi".
func Refs(c Commit) (mentioned []int, closed []int) {
	text := c.Subject + "\n" + c.Body
	mentioned = uniqueInts(issueRef.FindAllStringSubmatch(text, -1))
	var cl [][]string
	for _, m := range closeRef.FindAllStringSubmatch(text, -1) {
		cl = append(cl, issueRef.FindAllStringSubmatch(m[1], -1)...)
	}
	closed = uniqueInts(cl)
	return mentioned, closed
}

func uniqueInts(ms [][]string) []int {
	seen := map[int]bool{}
	var out []int
	for _, m := range ms {
		n, err := strconv.Atoi(m[1])
		if err != nil || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

// ParseGitLog — `git log --format=%H%x1f%s%x1f%b%x1e` chiqishini o'qiydi.
func ParseGitLog(out string) []Commit {
	var cs []Commit
	for _, rec := range strings.Split(out, "\x1e") {
		rec = strings.TrimLeft(rec, "\r\n")
		if rec == "" {
			continue
		}
		f := strings.SplitN(rec, "\x1f", 3)
		if len(f) < 2 {
			continue
		}
		c := Commit{SHA: strings.TrimSpace(f[0]), Subject: strings.TrimSpace(f[1])}
		if len(f) == 3 {
			c.Body = strings.TrimSpace(f[2])
		}
		cs = append(cs, c)
	}
	return cs
}

// ── Prod log xatolari ────────────────────────────────────────────────

// LogError — bir xil turdagi xatolar guruhi.
type LogError struct {
	Sig    string // barqaror imzo (raqam/ID'larsiz)
	Title  string
	Sample string // maxfiy ma'lumoti niqoblangan namuna
	Count  int
}

var (
	slogKV   = regexp.MustCompile(`(\w+)=("(?:[^"\\]|\\.)*"|\S+)`)
	reHexID  = regexp.MustCompile(`\b[0-9a-f]{8,}\b`)
	reUUID   = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)
	reNum    = regexp.MustCompile(`\d+`)
	rePhone  = regexp.MustCompile(`\+?998\d{9}`)
	reEmail  = regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.]+`)
	reBearer = regexp.MustCompile(`(?i)(bearer|token|key|secret|password|passwd|pwd)([=:\s"]+)[^\s"&]+`)
	reIP     = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`)
)

// Mask — logdan Plane'ga chiqadigan matndagi shaxsiy/maxfiy qiymatlarni yopadi.
func Mask(s string) string {
	s = reBearer.ReplaceAllString(s, "$1$2***")
	s = rePhone.ReplaceAllString(s, "+998*********")
	s = reEmail.ReplaceAllString(s, "***@***")
	s = reIP.ReplaceAllString(s, "*.*.*.*")
	return s
}

func normalize(s string) string {
	s = reUUID.ReplaceAllString(s, "<id>")
	s = reHexID.ReplaceAllString(s, "<id>")
	return reNum.ReplaceAllString(s, "N")
}

// GroupErrors — slog matn loglaridan level=ERROR va panic qatorlarini imzo bo'yicha guruhlaydi.
func GroupErrors(logs string) []LogError {
	groups := map[string]*LogError{}
	var order []string
	for _, line := range strings.Split(logs, "\n") {
		title, ok := errorTitle(line)
		if !ok {
			continue
		}
		sum := sha256.Sum256([]byte(title))
		sig := hex.EncodeToString(sum[:])[:12]
		g := groups[sig]
		if g == nil {
			g = &LogError{Sig: sig, Title: title, Sample: Mask(truncate(line, 600))}
			groups[sig] = g
			order = append(order, sig)
		}
		g.Count++
	}
	out := make([]LogError, 0, len(order))
	for _, s := range order {
		out = append(out, *groups[s])
	}
	return out
}

func errorTitle(line string) (string, bool) {
	if strings.HasPrefix(line, "panic:") {
		return "panic: " + truncate(normalize(Mask(strings.TrimPrefix(line, "panic:"))), 120), true
	}
	if !strings.Contains(line, "level=ERROR") {
		return "", false
	}
	kv := map[string]string{}
	for _, m := range slogKV.FindAllStringSubmatch(line, -1) {
		kv[m[1]] = strings.Trim(m[2], `"`)
	}
	title := kv["msg"]
	if e := kv["err"]; e != "" {
		title += " — " + truncate(normalize(Mask(e)), 100)
	} else if e := kv["error"]; e != "" {
		title += " — " + truncate(normalize(Mask(e)), 100)
	}
	if title == "" {
		return "", false
	}
	return truncate(title, 200), true
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
