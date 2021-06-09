// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.16
// +build go1.16

package main

import (
	"encoding/json"
	"go/format"
	"io/fs"
	"log"
	"net/http"
	pathpkg "path"
	"strings"

	"golang.org/x/website/internal/env"
	"golang.org/x/website/internal/godoc"
	"golang.org/x/website/internal/redirect"
)

var (
	pres *godoc.Presentation
	fsys fs.FS
)

// toFS returns the io/fs name for path (no leading slash).
func toFS(path string) string {
	if path == "/" {
		return "."
	}
	return pathpkg.Clean(strings.TrimPrefix(path, "/"))
}

// hostEnforcerHandler redirects requests to "http://foo.golang.org/bar"
// to "https://golang.org/bar".
// It permits requests to the host "godoc-test.golang.org" for testing and
// golang.google.cn for Chinese users.
type hostEnforcerHandler struct {
	h http.Handler
}

func (h hostEnforcerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !env.EnforceHosts() {
		h.h.ServeHTTP(w, r)
		return
	}
	if !h.isHTTPS(r) || !h.validHost(r.Host) {
		r.URL.Scheme = "https"
		if h.validHost(r.Host) {
			r.URL.Host = r.Host
		} else {
			r.URL.Host = "golang.org"
		}
		http.Redirect(w, r, r.URL.String(), http.StatusFound)
		return
	}
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
	h.h.ServeHTTP(w, r)
}

func (h hostEnforcerHandler) isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func (h hostEnforcerHandler) validHost(host string) bool {
	switch strings.ToLower(host) {
	case "golang.org", "golang.google.cn":
		return true
	}
	if strings.HasSuffix(host, "-dot-golang-org.appspot.com") {
		// staging/test
		return true
	}
	return false
}

func registerHandlers(pres *godoc.Presentation) *http.ServeMux {
	if pres == nil {
		panic("nil Presentation")
	}
	mux := http.NewServeMux()
	mux.Handle("/", pres)
	mux.Handle("/doc/codewalk/", http.HandlerFunc(codewalk))
	mux.Handle("/fmt", http.HandlerFunc(fmtHandler))
	mux.Handle("/x/", http.HandlerFunc(xHandler))
	redirect.Register(mux)

	http.Handle("/", hostEnforcerHandler{mux})

	return mux
}

func readTemplates(p *godoc.Presentation) {
	var err error
	if codewalkHTML, err = p.ReadTemplate("codewalk.html"); err != nil {
		log.Fatal(err)
	}
	if codewalkdirHTML, err = p.ReadTemplate("codewalkdir.html"); err != nil {
		log.Fatal(err)
	}
}

type fmtResponse struct {
	Body  string
	Error string
}

// fmtHandler takes a Go program in its "body" form value, formats it with
// standard gofmt formatting, and writes a fmtResponse as a JSON object.
func fmtHandler(w http.ResponseWriter, r *http.Request) {
	resp := new(fmtResponse)
	body, err := format.Source([]byte(r.FormValue("body")))
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Body = string(body)
	}
	w.Header().Set("Content-type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(resp)
}

func blogHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://docs.studygolang.com/blog"+strings.TrimPrefix(r.URL.Path, "/blog"), http.StatusFound)
}
