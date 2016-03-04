package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"path"
	"strings"
	"time"
)

type RedirectServer struct {
	path string
}

func (s *RedirectServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	base := path.Base(req.URL.Path)
	if base == "" || strings.Contains(base, "..") {
		http.Error(w, "INVALID REQUEST", http.StatusBadRequest)
		return
	}

	if req.Header.Get("X-Purpose") == "preview" {
		base += ".preview"
	}

	filename := path.Join(s.path, base)

	body, err := ioutil.ReadFile(filename)
	if err != nil {
		switch {
		case os.IsNotExist(err):
			log.Printf("%q %s", filename, err)
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		default:
			log.Printf("%s %s", req.RequestURI, err)
			http.Error(w, "Error", http.StatusInternalServerError)
			return
		}
	}

	err = dumpReq(fmt.Sprintf("%s.%s", filename, time.Now().Format("2006-01-02_15-04-05.000000")), req)
	if err != nil {
		log.Printf("%s %s", req.RequestURI, err)
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}

	var location string
	var code int
	switch {
	case strings.HasPrefix(string(body), "301 "):
		code = 301
		location = strings.TrimSpace(string(body)[4:])
		body = nil
	case strings.HasPrefix(string(body), "302 "):
		code = 302
		location = strings.TrimSpace(string(body)[4:])
		body = nil
	case strings.HasPrefix(string(body), "307 "):
		code = 307
		location = strings.TrimSpace(string(body)[4:])
		body = nil
	case strings.HasPrefix(string(body), "403"):
		code = 403
		body = nil
	case strings.HasPrefix(string(body), "429"):
		code = 429
		body = nil
	case strings.HasPrefix(string(body), "500"):
		code = 500
		body = nil
	case strings.HasPrefix(string(body), "503"):
		code = 503
		body = nil
	default:
		code = 200
	}

	log.Printf("%d %s %q %q", code, req.Method, req.RequestURI, req.UserAgent())

	switch code {
	case 301, 302:
		http.Redirect(w, req, location, code)
		return
	case 500:
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}
	// assume this is text/html
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if req.Method == "GET" {
		w.Write(body)
	}
}

func dumpReq(filename string, r *http.Request) error {
	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		return err
	}
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	_, err = f.Write(dump)
	return err
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	addr := flag.String("http", ":http", "address to listen on")
	path := flag.String("path", "", "base path for redirect data")
	flag.Parse()

	if *path == "" {
		log.Fatalf("--path required")
	}

	s := &http.Server{
		Addr:    *addr,
		Handler: &RedirectServer{*path},
	}
	err := s.ListenAndServe()
	if err != nil {
		log.Fatalf("%s", err)
	}

}
