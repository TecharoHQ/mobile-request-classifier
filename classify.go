package mobilerequestclassifier

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	phone_classifier "github.com/TecharoHQ/mobile-request-classifier/phone_classifier"
	"github.com/TecharoHQ/secchua"
	"github.com/mssola/useragent"
)

//go:generate make rust

type RequestLikelihood struct {
	IsSlow  bool `json:"is_slow"`
	IsPhone bool `json:"is_phone"`
	IsBot   bool `json:"is_bot"`
}

type key int

const requestLikelihoodKey key = iota

// Store client classification results into a context.Context.
func Store(ctx context.Context, likelihood *RequestLikelihood) context.Context {
	return context.WithValue(ctx, requestLikelihoodKey, likelihood)
}

// Get client classification results from a context.Context, nil if not discovered.
func Get(ctx context.Context) *RequestLikelihood {
	if v, ok := ctx.Value(requestLikelihoodKey).(*RequestLikelihood); ok {
		return v
	}

	return nil
}

// Middleware wraps an HTTP handler, annotating client requests with the mobile phone
// request likelihood for downstream services.
type Middleware struct {
	pcf  *phone_classifier.ClassifierFactory
	next http.Handler
	lg   *slog.Logger
}

// New constructs a new classification middleware instance.
func New(ctx context.Context, lg *slog.Logger, next http.Handler, wasmBytes []byte) (*Middleware, error) {
	pcf, err := phone_classifier.NewPhoneClassifierFactory(ctx, wasmBytes)
	if err != nil {
		return nil, err
	}

	return &Middleware{
		pcf:  pcf,
		next: next,
		lg:   lg,
	}, nil
}

// ServeHTTP wraps the downstream handler with client prediction results.
//
// If any step of this fails, the entire attempt fails, an error is logged, and
// the next part of the middleware chain is called as normal.
func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.pcf == nil {
		m.next.ServeHTTP(w, r)
		return
	}

	r, err := m.serveHTTP(w, r)
	if err != nil {
		m.lg.ErrorContext(r.Context(), "can't classify incoming request", "err", err)
	}

	m.next.ServeHTTP(w, r)
}

func (m *Middleware) serveHTTP(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
	clientHints, err := secchua.ParseClient(r)
	if err != nil {
		return r, err
	}

	ua := useragent.New(r.UserAgent())

	// TODO(Xe): move this to a pool?
	pci, err := m.pcf.Instantiate(r.Context())
	if err != nil {
		return r, err
	}
	defer pci.Close(r.Context())

	proto := phone_classifier.HttpVersion{Major: 254, Minor: 254}

	if hdr := r.Header.Get("X-Http-Protocol"); hdr != "" {
		switch strings.ToLower(hdr) {
		case "http/1.1":
			proto = phone_classifier.HttpVersion{Major: 1, Minor: 1}
		case "http/2.0":
			proto = phone_classifier.HttpVersion{Major: 2, Minor: 0}
		case "http/3.0":
			proto = phone_classifier.HttpVersion{Major: 3, Minor: 0}
		}
	}

	bname, bver := ua.Browser()
	browserVersion := phone_classifier.Version{Name: bname, Version: bver}

	ename, ever := ua.Engine()
	engineVersion := phone_classifier.Version{Name: ename, Version: ever}

	osi := ua.OSInfo()
	osInfo := phone_classifier.OsInfo{
		FullName: osi.FullName,
		Name:     osi.Name,
		Version:  osi.Version,
	}

	var schua *phone_classifier.SecChUa

	if clientHints != nil {
		schua = &phone_classifier.SecChUa{
			Mobile:   clientHints.Mobile,
			Platform: clientHints.Platform,
			Arch:     clientHints.Arch,
			Bitness:  clientHints.Bitness,
		}

		for _, ver := range clientHints.Versions {
			schua.Versions = append(schua.Versions, phone_classifier.Version{
				Name:    ver.Name,
				Version: ver.Version,
			})
		}
	}

	mobileGuess, err := pci.IsMobile(r.Context(), phone_classifier.HttpRequest{
		Method: r.Method,
		Path:   r.URL.Path,
		Query:  r.URL.Query().Encode(),
		Proto:  proto,
		UserAgent: phone_classifier.UserAgent{
			Bot:          ua.Bot(),
			Browser:      browserVersion,
			Engine:       engineVersion,
			Localization: ua.Localization(),
			Mobile:       ua.Mobile(),
			Model:        ua.Model(),
			Os:           ua.OS(),
			OsInfo:       osInfo,
			Platform:     ua.Platform(),
			Orig:         r.UserAgent(),
		},
		SecChUa: schua,
	})
	if err != nil {
		return r, err
	}

	r = r.WithContext(Store(r.Context(), &RequestLikelihood{
		IsSlow:  mobileGuess.IsSlow,
		IsPhone: mobileGuess.IsPhone,
		IsBot:   mobileGuess.IsBot,
	}))

	return r, nil
}
