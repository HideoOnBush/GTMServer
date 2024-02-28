package tracer

import (
	"GTMServer/quicClient"
	"GTMServer/utils"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

var BPTracer *Tracer
var Once sync.Once

type Tracer struct {
	*chTrace
	*span
	*quicClient.QClient
	timer *time.Ticker
}

type ConfigTracer struct {
	ChBuffSize  int
	FlushTime   time.Duration
	ServiceName string
}

type chTrace struct {
	ch          chan string
	initialized bool
}

type span struct {
	sourceServiceMap *sync.Map
	spanName         string
	spanScene        string
	spanCore         bool
}

func InitTracer(config *ConfigTracer, client *quicClient.QClient) {
	t := Tracer{}
	t.chTrace = &chTrace{}
	t.ch = make(chan string, config.ChBuffSize)
	t.initialized = true
	var sMap sync.Map
	t.span = &span{
		sourceServiceMap: &sMap,
		spanName:         config.ServiceName,
		spanScene:        utils.GetSceneFromName(config.ServiceName),
		spanCore:         utils.GetCoreFromName(config.ServiceName),
	}
	timer := time.NewTicker(config.FlushTime)
	t.timer = timer
	t.QClient = client
	Once.Do(func() {
		BPTracer = &t
		go t.saveTraceData(context.Background())
		go t.settleTraceData(context.Background())
	})
}

func (t *Tracer) saveTraceData(ctx context.Context) {
	if !t.initialized {
		log.Fatalf("ch in Tracer haven't init")
	}
	for sourceServer := range t.ch {
		_, _ = t.sourceServiceMap.LoadOrStore(sourceServer, struct{}{})
	}
}

func (t *Tracer) settleTraceData(ctx context.Context) {
	defer t.timer.Stop()
	for _ = range t.timer.C {
		var sourceService []string = make([]string, 0)
		t.sourceServiceMap.Range(func(key, _ interface{}) bool {
			sourceService = append(sourceService, key.(string))
			t.sourceServiceMap.Delete(key)
			return true
		})
		if len(sourceService) == 0 {
			continue
		}
		var lines []*quicClient.Line = make([]*quicClient.Line, 0, len(sourceService))
		for _, name := range sourceService {
			lines = append(lines, &quicClient.Line{
				Source:       name,
				SourceIsCore: utils.GetCoreFromName(name),
				SourceScene:  utils.GetSceneFromName(name),
				Target:       t.spanName,
				TargetIsCore: t.spanCore,
				TargetScene:  t.spanScene,
				Dependence:   "",
			})
		}
		data, err := json.Marshal(lines)
		if err != nil {
			log.Fatalf("Marshal in settleTraceData failed,err=%v", err)
		}
		err = t.QClient.DialQuic(ctx, data)
		if err != nil {
			log.Fatalf("DialQuic in settleTraceData failed,err=%v", err)
		}
	}
}

func TracerTopology(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastServer := r.Header.Get("Last-Span-Name")
		if lastServer != "" {
			BPTracer.ch <- lastServer
		}
		next.ServeHTTP(w, r)
	})
}
