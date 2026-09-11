package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// lokiCore, TUM WithAttrs/WithGroup kopyalari arasinda PAYLASILAN durumdur
// (HTTP client, kanal, arka plan goroutine). LokiHandler bunu bir ISARETCI
// olarak tutar; boylece slog'un "With... yeni bir handler dondur" kuralini
// (WithAttrs/WithGroup) uygularken sync.WaitGroup/sync.Once gibi KOPYALANAMAZ
// alanlari kopyalamis olmayiz -- `go vet` bu hatayi yakalar.
type lokiCore struct {
	url     string // or. http://localhost:3100/loki/api/v1/push
	service string // "service" etiketinin degeri, or. "ticketsale-api"
	client  *http.Client

	entries chan entry
	wg      sync.WaitGroup

	warnOnce sync.Once // Loki'ye ulasilamiyorsa uyariyi bir kere yaz, log firtinasi yaratma
}

// LokiHandler kayitlari Grafana Loki'nin HTTP push API'sine (POST /loki/api/v1/push)
// yollayan bir slog.Handler'dir. Kendisi kucuk ve KOPYALANABILIR: paylasilan
// core'a bir isaretci + bu ozel zincirin (With... ile eklenmis) attrs/groups'u.
//
// TASARIM KARARLARI:
//
//  1. Loki "label" (etiket) ile "log govdesi"ni ayirir. Etiketler INDEKSLENIR --
//     az sayida, DUSUK KARDINALITELI degerler olmali (or. "service", "event").
//     seat_id / user_id / order_id gibi neredeyse SONSUZ farkli deger alabilen
//     alanlari etiket yaparsan Loki'nin index'i patlar ("cardinality explosion").
//     Bu yuzden sadece "service" ve "event" (ornegin "seat_held") etiket olur;
//     seat_id/user_id/order_id JSON govdenin ICINDE kalir, LogQL'de
//     `| json | user_id="..."` ile filtrelenir.
//
//  2. ASENKRON VE NON-BLOCKING: Handle() bir kanala yazar ve HEMEN doner.
//     Loki cevap vermese, hatta hic ayakta olmasa bile istegi isleyen goroutine
//     asla beklemez. Arka plandaki bir goroutine periyodik olarak biriken
//     kayitlari toplu (batch) halde gonderir. Kanal dolarsa YENI kayitlar
//     SESSIZCE ATLANIR -- loglama, uygulamanin ana isini asla yavaslatmamali
//     ya da durdurmamali.
type LokiHandler struct {
	core   *lokiCore
	attrs  []slog.Attr // WithAttrs ile eklenmis sabit alanlar
	groups []string    // WithGroup ile eklenmis grup adlari (anahtar on eki icin)
}

type entry struct {
	labels map[string]string
	ts     time.Time
	line   string
}

// NewLokiHandler bir LokiHandler kurar ve arka plan gonderici goroutine'ini baslatir.
// url bos ise handler hicbir sey yapmaz (no-op) -- Loki'yi kapatmak istersen
// LOKI_URL'i bos birak.
func NewLokiHandler(ctx context.Context, url, service string) *LokiHandler {
	core := &lokiCore{
		url:     strings.TrimSuffix(url, "/") + "/loki/api/v1/push",
		service: service,
		client:  &http.Client{Timeout: 5 * time.Second},
		entries: make(chan entry, 1000), // 1000 kayitlik tampon; dolarsa yenileri atla
	}
	h := &LokiHandler{core: core}
	if url == "" {
		return h
	}
	core.wg.Add(1)
	go core.run(ctx)
	return h
}

func (h *LokiHandler) Enabled(context.Context, slog.Level) bool { return h.core.url != "" }

func (h *LokiHandler) Handle(_ context.Context, r slog.Record) error {
	if h.core.url == "" {
		return nil
	}

	prefix := ""
	if len(h.groups) > 0 {
		prefix = strings.Join(h.groups, ".") + "."
	}

	fields := make(map[string]any, r.NumAttrs()+len(h.attrs)+2)
	fields["msg"] = r.Message
	fields["level"] = r.Level.String()
	for _, a := range h.attrs {
		fields[prefix+a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		fields[prefix+a.Key] = a.Value.Any()
		return true
	})

	// "event" alani varsa DUSUK KARDINALITE oldugunu varsayip etiket yapiyoruz
	// (bkz. yukaridaki tip yorumu). Yoksa sadece "service" etiketiyle gider.
	labels := map[string]string{"service": h.core.service}
	if ev, ok := fields["event"].(string); ok && ev != "" {
		labels["event"] = ev
	}

	line, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("loki: kayit json'a cevrilemedi: %w", err)
	}

	select {
	case h.core.entries <- entry{labels: labels, ts: r.Time, line: string(line)}:
	default:
		// Tampon dolu -- bu kaydi ATLA. Loglama ugruna ana akisi bloklamayiz.
	}
	return nil
}

func (h *LokiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LokiHandler{
		core:   h.core,
		attrs:  append(append([]slog.Attr{}, h.attrs...), attrs...),
		groups: h.groups,
	}
}

func (h *LokiHandler) WithGroup(name string) slog.Handler {
	return &LokiHandler{
		core:   h.core,
		attrs:  h.attrs,
		groups: append(append([]string{}, h.groups...), name),
	}
}

// run arka planda calisir: periyodik olarak biriken kayitlari toplu gonderir.
func (c *lokiCore) run(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var pending []entry
	flush := func() {
		if len(pending) == 0 {
			return
		}
		c.push(pending)
		pending = pending[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case e := <-c.entries:
			pending = append(pending, e)
			if len(pending) >= 100 { // buyuk bir birikme olursa erken bosalt
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// push, ayni etiket setine sahip kayitlari ayni "stream" altinda toplayip
// Loki'nin bekledigi govdeyi (streams[].values[][ts, line]) POST eder.
func (c *lokiCore) push(entries []entry) {
	streams := map[string]*lokiStream{}
	for _, e := range entries {
		key := labelKey(e.labels)
		s, ok := streams[key]
		if !ok {
			s = &lokiStream{Stream: e.labels}
			streams[key] = s
		}
		// Loki, bir stream icindeki degerlerin ZAMANA GORE ARTAN sirada
		// olmasini bekler; kanaldan geldigi sira zaten kronolojiktir.
		s.Values = append(s.Values, [2]string{
			strconv.FormatInt(e.ts.UnixNano(), 10), e.line,
		})
	}

	body := lokiPushBody{Streams: make([]*lokiStream, 0, len(streams))}
	for _, s := range streams {
		body.Streams = append(body.Streams, s)
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, c.url, bytes.NewReader(buf))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		c.warnOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "loki: push basarisiz (%s) -- Grafana loglari bu ana kadar gorunmeyecek, uygulama calismaya devam ediyor\n", err)
		})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		c.warnOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "loki: push %d dondu\n", resp.StatusCode)
		})
	}
}

func labelKey(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
		b.WriteByte(',')
	}
	return b.String()
}

type lokiPushBody struct {
	Streams []*lokiStream `json:"streams"`
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"`
}
