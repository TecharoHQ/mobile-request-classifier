package mobilerequestclassifier

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/neilotoole/slogt/v2"
)

//go:embed var/mobile_request_classifier.wasm
var prebuilt []byte

func TestBasicFunctionality(t *testing.T) {
	lg := slogt.New(t)
	classifier, err := New(t.Context(), lg, http.DefaultServeMux, prebuilt)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(classifier)
	defer srv.Close()

	_, err = srv.Client().Get("http://example.com/")
	if err != nil {
		t.Fatal(err)
	}
}
