// Command planesync â€” Plane vazifalarini kod va prod bilan avtomatik moslaydi
// (ONDEXECOSY-505). Qo'lda holat o'zgartirish shart emas.
//
//	planesync commits --range A..B   CI (push main): commit eslatgan vazifa ->
//	                                 izoh + Todo/Backlog bo'lsa "In Progress";
//	                                 "Tugadi: ONDEXECOSY-N" -> "Review" (kod tayyor).
//	planesync deploy --sha S         deploy muvaffaqiyatli: S gacha bo'lgan
//	                                 "Tugadi:" vazifalari Review -> "Done".
//	planesync errors --container C   server cron: prod logidagi yangi xato turi ->
//	                                 Plane bug (dublsiz); Done bo'lgan xato qaytsa -> qayta ochiladi.
//
// Kalit: PLANE_API_KEY yoki PLANE_API_KEY_FILE. Hech qachon chop etilmaydi.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultBase    = "https://plane.ondex.uz/api/v1/workspaces/ondex"
	defaultProject = "c4df293c-e86d-49c8-a4df-9e75c753a6d3"
	defaultRepo    = "https://github.com/Shoxake17/OnDexSuperApp"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		log.Fatal("foydalanish: planesync commits|deploy|errors [bayroqlar]")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	p, err := newPlane()
	if err != nil {
		log.Fatal(err)
	}
	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "commits":
		err = runCommits(ctx, p, args)
	case "deploy":
		err = runDeploy(ctx, p, args)
	case "errors":
		err = runErrors(ctx, p, args)
	default:
		err = fmt.Errorf("noma'lum buyruq: %s", cmd)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func newPlane() (*Plane, error) {
	key := strings.TrimSpace(os.Getenv("PLANE_API_KEY"))
	if key == "" {
		if f := os.Getenv("PLANE_API_KEY_FILE"); f != "" {
			b, err := os.ReadFile(filepath.Clean(f)) // #nosec G304 G703 -- operator bergan yo'l
			if err != nil {
				return nil, err
			}
			key = strings.TrimSpace(string(b))
		}
	}
	if key == "" {
		return nil, errors.New("PLANE_API_KEY yoki PLANE_API_KEY_FILE kerak")
	}
	return &Plane{
		Base: env("PLANE_BASE", defaultBase), Project: env("PLANE_PROJECT", defaultProject),
		Key: key, HTTP: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func gitLog(args ...string) ([]Commit, error) {
	full := append([]string{"log", "--format=%H%x1f%s%x1f%b%x1e"}, args...)
	out, err := exec.Command("git", full...).Output() // #nosec G204 -- argumentlar CI'dan
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	return ParseGitLog(string(out)), nil
}

func commitLink(sha string) string {
	repo := env("REPO_URL", defaultRepo)
	return fmt.Sprintf(`<a href="%s/commit/%s">%s</a>`, repo, sha, sha[:7])
}

// â”€â”€ commits â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func runCommits(ctx context.Context, p *Plane, args []string) error {
	fs := flag.NewFlagSet("commits", flag.ExitOnError)
	rng := fs.String("range", "", "git oralig'i, masalan BEFORE..AFTER")
	_ = fs.Parse(args)
	if *rng == "" {
		return errors.New("--range kerak")
	}
	cs, err := gitLog("--reverse", *rng)
	if err != nil {
		return err
	}
	if err := p.Load(ctx); err != nil {
		return err
	}
	for _, c := range cs {
		if err := applyCommit(ctx, p, c); err != nil {
			log.Printf("ogohlantirish: %s: %v", c.SHA[:7], err)
		}
	}
	return nil
}

func applyCommit(ctx context.Context, p *Plane, c Commit) error {
	mentioned, closed := Refs(c)
	closing := map[int]bool{}
	for _, n := range closed {
		closing[n] = true
	}
	for _, n := range mentioned {
		i, st := p.StateOf(n)
		if st == "" || st == "Cancelled" {
			continue
		}
		if dup, err := p.HasComment(ctx, i, c.SHA[:7]); err != nil || dup {
			continue
		}
		note := "commit " + commitLink(c.SHA) + ": " + html.EscapeString(c.Subject)
		target := ""
		switch {
		case closing[n] && st != "Done":
			target = "Review"
			note += "<br>Kod tayyor â€” prod deploy kutilmoqda (avtomatik)."
		case st == "Backlog" || st == "Todo":
			target = "In Progress"
		}
		if target != "" && target != st {
			if err := p.SetState(ctx, i, target); err != nil {
				return err
			}
			note += fmt.Sprintf("<br>Holat: %s â†’ %s", st, target)
		}
		if err := p.Comment(ctx, i, "<p>"+note+"</p>"); err != nil {
			return err
		}
		log.Printf("ONDEXECOSY-%d: %s %s", n, c.SHA[:7], target)
	}
	return nil
}

// â”€â”€ deploy â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func runDeploy(ctx context.Context, p *Plane, args []string) error {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	sha := fs.String("sha", "", "prod'ga chiqqan commit")
	depth := fs.Int("depth", 400, "nechta oldingi commit ko'riladi")
	what := fs.String("what", "prod", "nima deploy qilindi (izoh uchun)")
	_ = fs.Parse(args)
	if *sha == "" {
		return errors.New("--sha kerak")
	}
	cs, err := gitLog("-n", fmt.Sprint(*depth), *sha)
	if err != nil {
		return err
	}
	if err := p.Load(ctx); err != nil {
		return err
	}
	for _, c := range cs {
		_, closed := Refs(c)
		for _, n := range closed {
			i, st := p.StateOf(n)
			if st != "Review" {
				continue
			}
			if err := p.SetState(ctx, i, "Done"); err != nil {
				return err
			}
			msg := fmt.Sprintf("<p>Prod'ga chiqdi (%s, %s) â€” avtomatik Done. Yopgan commit: %s</p>",
				html.EscapeString(*what), (*sha)[:7], commitLink(c.SHA))
			if err := p.Comment(ctx, i, msg); err != nil {
				return err
			}
			log.Printf("ONDEXECOSY-%d -> Done", n)
		}
	}
	return nil
}

// â”€â”€ errors â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

type errState struct {
	Seq      int    `json:"seq"`
	LastNote string `json:"last_note"` // YYYY-MM-DD â€” kuniga bittadan ko'p izoh yozilmaydi
	Total    int    `json:"total"`
}

func runErrors(ctx context.Context, p *Plane, args []string) error {
	fs := flag.NewFlagSet("errors", flag.ExitOnError)
	container := fs.String("container", "ondex-api-1", "docker konteyner")
	since := fs.String("since", "1h", "docker logs --since")
	statePath := fs.String("state", "/var/lib/planesync/errors.json", "imzo -> vazifa xaritasi")
	dry := fs.Bool("dry-run", false, "Plane'ga yozmaydi, faqat ko'rsatadi")
	_ = fs.Parse(args)

	out, err := exec.Command("docker", "logs", "--since", *since, *container).CombinedOutput() // #nosec G204
	if err != nil {
		return fmt.Errorf("docker logs: %w", err)
	}
	groups := GroupErrors(string(out))
	if *dry {
		for _, g := range groups {
			fmt.Printf("%dx [%s] %s\n", g.Count, g.Sig, g.Title)
		}
		return nil
	}
	if len(groups) == 0 {
		return nil
	}
	state := map[string]errState{}
	if b, err := os.ReadFile(*statePath); err == nil { // #nosec G304
		_ = json.Unmarshal(b, &state)
	}
	if err := p.Load(ctx); err != nil {
		return err
	}
	label := p.LabelID(ctx, "backend")
	for _, g := range groups {
		st, err := applyError(ctx, p, g, state[g.Sig], *container, label)
		if err != nil {
			log.Printf("ogohlantirish: %s: %v", g.Sig, err)
			continue
		}
		state[g.Sig] = st
	}
	b, _ := json.MarshalIndent(state, "", "  ")
	return os.WriteFile(*statePath, b, 0o600)
}

func applyError(ctx context.Context, p *Plane, g LogError, prev errState, container, label string) (errState, error) {
	today := time.Now().UTC().Format("2006-01-02")
	sample := "<pre>" + html.EscapeString(g.Sample) + "</pre>"
	i, st := p.StateOf(prev.Seq)
	if prev.Seq == 0 || st == "" {
		desc := fmt.Sprintf("<p>Prod logida yangi xato turi (avtomatik, %s). Imzo: <code>%s</code>. Oxirgi davrda: %d marta.</p>%s",
			html.EscapeString(container), g.Sig, g.Count, sample)
		var labels []string
		if label != "" {
			labels = []string{label}
		}
		ni, err := p.Create(ctx, "[prod xato] "+g.Title, desc, "Todo", "high", labels)
		if err != nil {
			return prev, err
		}
		log.Printf("yangi bug ONDEXECOSY-%d: %s", ni.Seq, g.Title)
		return errState{Seq: ni.Seq, LastNote: today, Total: g.Count}, nil
	}
	prev.Total += g.Count
	switch {
	case st == "Cancelled": // egasi "e'tiborsiz" degan
		return prev, nil
	case st == "Done":
		if err := p.SetState(ctx, i, "Todo"); err != nil {
			return prev, err
		}
		prev.LastNote = today
		return prev, p.Comment(ctx, i, fmt.Sprintf("<p>Xato QAYTDI (%dx) â€” avtomatik qayta ochildi.</p>%s", g.Count, sample))
	case prev.LastNote != today:
		prev.LastNote = today
		return prev, p.Comment(ctx, i, fmt.Sprintf("<p>Hali ham takrorlanmoqda: %dx (jami %d).</p>", g.Count, prev.Total))
	}
	return prev, nil
}
