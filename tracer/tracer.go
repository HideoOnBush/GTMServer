package tracer

import (
	"GTMServer/etcd"
	"GTMServer/quicClient"
	"GTMServer/quicClient/model"
	"context"
	"encoding/json"
	"errors"
	"github.com/golang/protobuf/proto"
	clientv3 "go.etcd.io/etcd/client/v3"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var BPTracer *Tracer
var Once sync.Once

type Tracer struct {
	*etcd.EtcdClient
	*chTrace
	*serverData
	*quicClient.QClient
	timer *time.Ticker
}

type Span struct {
	ServiceName  string `json:"serviceName"`
	ServiceScene string `json:"serviceScene"`
	Relation     string `json:"relation"`
	ServiceCore  bool   `json:"serviceCore"`
}

type ConfigTracer struct {
	ChBuffSize  int
	FlushTime   time.Duration
	ServiceName string
}

type chTrace struct {
	ch          chan *Span
	initialized bool
}

type serverData struct {
	sourceServiceMap *sync.Map
	serviceSpanMap   *sync.Map
	thisServerSpan   *Span
}

type tuple struct {
	name  string
	count int
}

func InitTracer(config *ConfigTracer, client *quicClient.QClient, etcdClient *etcd.EtcdClient) {
	t := Tracer{}
	t.chTrace = &chTrace{}
	t.EtcdClient = etcdClient
	t.ch = make(chan *Span, config.ChBuffSize)
	t.initialized = true
	var sourceServiceMap sync.Map
	var serviceSpanMap sync.Map
	//thisSpan, err := t.InitialServerAttr(config.ServiceName)
	//if err != nil {
	//	log.Fatalf("InitTracer failed,err=%v", err)
	//}
	thisSpan := &Span{
		ServiceName:  "GTMServerTom",
		ServiceScene: "test",
		Relation:     "weak",
		ServiceCore:  false,
	}
	t.serverData = &serverData{
		sourceServiceMap: &sourceServiceMap,
		serviceSpanMap:   &serviceSpanMap,
		thisServerSpan:   thisSpan,
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

func (t *Tracer) InitialServerAttr(serverName string) (s *Span, err error) {
	s = &Span{}
	prefix := "project"
	var resp *clientv3.GetResponse
	resp, err = t.Get(context.Background(), prefix, clientv3.WithPrefix())
	if err != nil {
		return
	}

	keyValues := resp.Kvs

	// 打印所有匹配的键值对
	flag := false
	for _, kv := range keyValues {
		parts := strings.Split(string(kv.Key), "/")
		if len(parts) >= 2 && parts[len(parts)-1] == serverName {
			err = json.Unmarshal(kv.Value, s)
			if err != nil {
				return
			}
			flag = true
		}
	}
	if !flag {
		err = errors.New("can't find server attributions")
	}
	return
}

func (t *Tracer) DiscoverService() (services map[uint32]map[uint16]string, err error) {
	prefix := "service"
	var resp *clientv3.GetResponse
	resp, err = t.Get(context.Background(), prefix, clientv3.WithPrefix())
	if err != nil {
		return
	}

	keyValues := resp.Kvs

	for _, kv := range keyValues {
		parts := strings.Split(string(kv.Key), "/")
		if len(parts) < 4 {
			sid, _ := strconv.Atoi(parts[2])
			pid, _ := strconv.Atoi(parts[3])
			services[uint32(sid)][uint16(pid)] = parts[1]
			log.Printf("dicover service From Etcd, ServiceName = %s,serviceId = %d,procedureId = %d", parts[1], parts[2], parts[3])
		}
	}
	return
}

func (t *Tracer) saveTraceData(ctx context.Context) {
	if !t.initialized {
		log.Fatalf("ch in Tracer haven't init")
	}
	for sourceServer := range t.ch {
		sourceServerName := sourceServer.ServiceName
		t.serviceSpanMap.LoadOrStore(sourceServerName, sourceServer)
		tempCount, ex := t.sourceServiceMap.Load(sourceServer)
		if ex {
			t.sourceServiceMap.Store(sourceServerName, tempCount.(int)+1)
		} else {
			t.sourceServiceMap.Store(sourceServerName, 1)
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
			sourceServiceSpan := value.(*Span)
			lines = append(lines, &model.Line{
				Source:       tup.name,
				SourceIsCore: sourceServiceSpan.ServiceCore,
				SourceScene:  sourceServiceSpan.ServiceScene,
				Target:       t.thisServerSpan.ServiceName,
				TargetIsCore: t.thisServerSpan.ServiceCore,
				TargetScene:  t.thisServerSpan.ServiceScene,
				Dependence:   sourceServiceSpan.Relation,
				VisitCount:   int64(tup.count),
			})
		}
		data, err := proto.Marshal(&model.LineList{Messages: lines})
		if err != nil {
			log.Fatalf("protoc marshal failed,err=%v", err)
		}
		_, err = t.QClient.DialQuic(ctx, data)
		if err != nil {
			log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Llongfile)
			log.Printf("send %d lines to Collector failed,errCode is %d,errMsg is %s", len(lines), "INVALID_ARGUMENT", "request ummarshal failed!")
		} else {
			log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Llongfile)
			log.Printf("send %d lines to Collector succeeded!", len(lines))
		}
	}
}

func TracerTopology(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Trace := r.Header.Get("Trace")
		if Trace != "" {
			log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Llongfile)
			lastSpan := Span{}
			err := json.Unmarshal([]byte(Trace), &lastSpan)
			if err != nil {
				log.Fatalf("Unmarshal of span failed,err=%v", err)
			}
			log.Printf("get service call from %s", lastSpan.ServiceName)
			BPTracer.ch <- &lastSpan
		}
		next.ServeHTTP(w, r)
	})
}
