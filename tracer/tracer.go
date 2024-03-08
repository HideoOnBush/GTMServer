package tracer

import (
	"GTMServer/quicClient"
	"GTMServer/quicClient/model"
	"GTMServer/utils"
	"context"
	"encoding/json"
	"github.com/golang/protobuf/proto"
	"log"
	"net/http"
	"sync"
	"time"
)

var BPTracer *Tracer
var Once sync.Once

type Tracer struct {
	*chTrace
	*serverData
	*quicClient.QClient
	timer *time.Ticker
}

type span struct {
	LastServiceName  string
	LastServiceScene string
	Relation         string
	LastServiceCore  bool
}

type ConfigTracer struct {
	ChBuffSize  int
	FlushTime   time.Duration
	ServiceName string
}

type chTrace struct {
	ch          chan *span
	initialized bool
}

type serverData struct {
	sourceServiceMap *sync.Map
	serviceSpanMap   *sync.Map
	serviceName      string
	serviceScene     string
	serviceCore      bool
}

type tuple struct {
	name  string
	count int
}

func InitTracer(config *ConfigTracer, client *quicClient.QClient) {
	t := Tracer{}
	t.chTrace = &chTrace{}
	t.ch = make(chan *span, config.ChBuffSize)
	t.initialized = true
	var sourceServiceMap sync.Map
	var serviceSpanMap sync.Map
	t.serverData = &serverData{
		sourceServiceMap: &sourceServiceMap,
		serviceSpanMap:   &serviceSpanMap,
		serviceName:      config.ServiceName,
		serviceScene:     utils.GetSceneFromName(config.ServiceName),
		serviceCore:      utils.GetCoreFromName(config.ServiceName),
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
		sourceServerName := sourceServer.LastServiceName
		t.serviceSpanMap.LoadOrStore(sourceServerName, sourceServer)
		tempCount, ex := t.sourceServiceMap.Load(sourceServer)
		if ex {
			t.sourceServiceMap.Store(sourceServer, tempCount.(int)+1)
		} else {
			t.sourceServiceMap.Store(sourceServer, 1)
		}
	}
}

func (t *Tracer) settleTraceData(ctx context.Context) {
	defer t.timer.Stop()
	for _ = range t.timer.C {
		var sourceService []tuple = make([]tuple, 0)
		t.sourceServiceMap.Range(func(key, value any) bool {
			sourceService = append(sourceService, tuple{
				name:  key.(string),
				count: value.(int),
			})
			t.sourceServiceMap.Delete(key)
			return true
		})
		if len(sourceService) == 0 {
			continue
		}
		//use json marshal
		//var lines []*quicClient.Line = make([]*quicClient.Line, 0, len(sourceService))
		//for _, tup := range sourceService {
		//	lines = append(lines, &quicClient.Line{
		//		Source:       tup.name,
		//		SourceIsCore: utils.GetCoreFromName(tup.name),
		//		SourceScene:  utils.GetSceneFromName(tup.name),
		//		Target:       t.spanName,
		//		TargetIsCore: t.spanCore,
		//		TargetScene:  t.spanScene,
		//		Dependence:   "",
		//		VisitCount:   int64(tup.count),
		//	})
		//}
		//data, err := json.Marshal(lines)
		//if err != nil {
		//	log.Fatalf("Marshal in settleTraceData failed,err=%v", err)
		//}
		var lines []*model.Line = make([]*model.Line, 0, len(sourceService))
		for _, tup := range sourceService {
			value, _ := t.serviceSpanMap.Load(tup.name)
			sourceServiceSpan := value.(*span)
			lines = append(lines, &model.Line{
				Source:       tup.name,
				SourceIsCore: sourceServiceSpan.LastServiceCore,
				SourceScene:  sourceServiceSpan.LastServiceScene,
				Target:       t.serviceName,
				TargetIsCore: t.serviceCore,
				TargetScene:  t.serviceScene,
				Dependence:   sourceServiceSpan.Relation,
				VisitCount:   int64(tup.count),
			})
		}
		data, err := proto.Marshal(&model.LineList{Messages: lines})
		if err != nil {
			log.Fatalf("protoc marshal failed,err=%v", err)
		}
		err = t.QClient.DialQuic(ctx, data)
		if err != nil {
			log.Fatalf("DialQuic in settleTraceData failed,err=%v", err)
		}
	}
}

func TracerTopology(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Trace := r.Header.Get("Trace")
		if Trace != "" {
			lastSpan := span{}
			err := json.Unmarshal([]byte(Trace), &lastSpan)
			if err != nil {
				log.Fatalf("Unmarshal of span failed,err=%v", err)
			}
			BPTracer.ch <- &lastSpan
		}
		next.ServeHTTP(w, r)
	})
}
