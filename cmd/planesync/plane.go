package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Plane — Plane REST API (v1) uchun kichik klient.
type Plane struct {
	Base, Project, Key string
	HTTP               *http.Client

	states map[string]string // nom -> id
	byID   map[string]string // id -> nom
	issues map[int]Issue
}

// Issue — vazifa (faqat kerakli maydonlar).
type Issue struct {
	ID    string `json:"id"`
	Seq   int    `json:"sequence_id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

func (p *Plane) do(ctx context.Context, method, path string, body, out any) error {
	url := path
	if !strings.HasPrefix(path, "http") {
		url = p.Base + "/projects/" + p.Project + path
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, url, rd)
		if err != nil {
			return err
		}
		req.Header.Set("X-API-Key", p.Key)
		req.Header.Set("Content-Type", "application/json")
		resp, err := p.HTTP.Do(req)
		if err != nil {
			return err
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			time.Sleep(15 * time.Second)
			if body != nil {
				b, _ := json.Marshal(body)
				rd = bytes.NewReader(b)
			}
			continue
		}
		if resp.StatusCode >= 300 {
			return fmt.Errorf("plane %s %s -> %d: %s", method, path, resp.StatusCode, truncate(string(data), 200))
		}
		if out != nil {
			return json.Unmarshal(data, out)
		}
		return nil
	}
}

type page[T any] struct {
	Results    []T    `json:"results"`
	NextCursor string `json:"next_cursor"`
	NextPage   bool   `json:"next_page_results"`
}

func listAll[T any](ctx context.Context, p *Plane, path string) ([]T, error) {
	var out []T
	cursor := ""
	for {
		u := path + "?per_page=100"
		if cursor != "" {
			u += "&cursor=" + cursor
		}
		var pg page[T]
		if err := p.do(ctx, http.MethodGet, u, nil, &pg); err != nil {
			return nil, err
		}
		out = append(out, pg.Results...)
		if !pg.NextPage || pg.NextCursor == "" {
			return out, nil
		}
		cursor = pg.NextCursor
	}
}

// Load — holatlar va vazifalarni bir marta o'qiydi.
func (p *Plane) Load(ctx context.Context) error {
	sts, err := listAll[struct{ ID, Name string }](ctx, p, "/states/")
	if err != nil {
		return err
	}
	p.states, p.byID = map[string]string{}, map[string]string{}
	for _, s := range sts {
		p.states[s.Name] = s.ID
		p.byID[s.ID] = s.Name
	}
	is, err := listAll[Issue](ctx, p, "/issues/")
	if err != nil {
		return err
	}
	p.issues = map[int]Issue{}
	for _, i := range is {
		p.issues[i.Seq] = i
	}
	return nil
}

// StateOf — vazifa holatining nomi ("" — vazifa topilmadi).
func (p *Plane) StateOf(seq int) (Issue, string) {
	i, ok := p.issues[seq]
	if !ok {
		return Issue{}, ""
	}
	return i, p.byID[i.State]
}

// SetState — holatni o'zgartiradi.
func (p *Plane) SetState(ctx context.Context, i Issue, state string) error {
	id, ok := p.states[state]
	if !ok {
		return fmt.Errorf("holat yo'q: %s", state)
	}
	if err := p.do(ctx, http.MethodPatch, "/issues/"+i.ID+"/", map[string]string{"state": id}, nil); err != nil {
		return err
	}
	i.State = id
	p.issues[i.Seq] = i
	return nil
}

// HasComment — vazifa izohlarida `needle` bormi (takroriy izohdan saqlanish).
func (p *Plane) HasComment(ctx context.Context, i Issue, needle string) (bool, error) {
	cs, err := listAll[struct {
		HTML string `json:"comment_html"`
	}](ctx, p, "/issues/"+i.ID+"/comments/")
	if err != nil {
		return false, err
	}
	for _, c := range cs {
		if strings.Contains(c.HTML, needle) {
			return true, nil
		}
	}
	return false, nil
}

// Comment — HTML izoh qo'shadi.
func (p *Plane) Comment(ctx context.Context, i Issue, html string) error {
	return p.do(ctx, http.MethodPost, "/issues/"+i.ID+"/comments/", map[string]string{"comment_html": html}, nil)
}

// Create — yangi vazifa.
func (p *Plane) Create(ctx context.Context, name, html, state, priority string, labels []string) (Issue, error) {
	body := map[string]any{"name": name, "description_html": html, "priority": priority, "state": p.states[state]}
	if len(labels) > 0 {
		body["labels"] = labels
	}
	var i Issue
	if err := p.do(ctx, http.MethodPost, "/issues/", body, &i); err != nil {
		return Issue{}, err
	}
	p.issues[i.Seq] = i
	return i, nil
}

// LabelID — yorliq id'si nomi bo'yicha ("" — yo'q).
func (p *Plane) LabelID(ctx context.Context, name string) string {
	ls, err := listAll[struct{ ID, Name string }](ctx, p, "/labels/")
	if err != nil {
		return ""
	}
	for _, l := range ls {
		if l.Name == name {
			return l.ID
		}
	}
	return ""
}
