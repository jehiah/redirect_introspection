package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"path"
	"strconv"
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
	code, err := strconv.Atoi(string(body[:3]))
	if err != nil {
		code = 200
	}

	// http://racksburg.com/choosing-an-http-status-code/
	switch code {
	case 301, 302, 303, 307, 308:
		location = strings.TrimSpace(string(body)[4:])
		body = nil
	case 300, 304, 305, 306:
		body = bytes.TrimSpace(body[3:])
		// use body (if any)
	case 200:
		// do nothing. use body (if any)
	case 201, 202, 203, 204, 205, 207, 208, 226:
		body = bytes.TrimSpace(body[3:])
		// use body (if any)
	case 400, 401, 402, 403, 404, 405, 406, 407, 408, 409, 410, 411, 412, 413, 414, 415, 416, 417, 418, 421, 422, 423, 426, 428, 429, 431:
		body = bytes.TrimSpace(body[3:])
	case 500, 501, 502, 503, 504, 505, 506, 507, 508, 510, 511:
		body = bytes.TrimSpace(body[3:])
	default:
		log.Printf("%d unknown response code", code)
		http.Error(w, "Error", http.StatusInternalServerError)
	}

	log.Printf("%d %s %q %q", code, req.Method, req.RequestURI, req.UserAgent())

	switch code {
	case 301, 302, 303, 307, 308:
		http.Redirect(w, req, location, code)
		return
	case 500, 501, 502, 503, 504, 505, 506, 507, 508, 510, 511:
		if body == nil {
			http.Error(w, "Error", code)
			return
		}
	}
	if body != nil {
		// assume this is text/html
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
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
