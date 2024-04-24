package quicClient

import (
	"GTMServer/quicClient/model"
	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	"sync"
	"testing"
	"time"
)

//func TestQuic_1(t *testing.T) {
//	a := model.LineReq{
//		Nums: 1,
//		Lines: &model.LineList{Messages: []*model.Line{
//			{
//				Source:       "test1",
//				SourceIsCore: false,
//				SourceScene:  "test",
//				Target:       "test2",
//				TargetIsCore: false,
//				TargetScene:  "test",
//				Dependence:   "weak",
//				VisitCount:   1,
//			},
//		}},
//	}
//	b := model.ErrorResp{Code: 4, Message: "service not found"}
//	t.Run("1", func(t *testing.T) {
//		data1, _ := proto.Marshal(&a)
//		aa := len(data1)
//		data2, _ := proto.Marshal(&b)
//		bb := len(data2)
//		log.Printf("request:{header:{MessageType:1 ServiceID:2 ProcedureID:1 PayLoadSize:%d} data:%v}, response:{header:{MessageType:2 ServiceID:0 ProcedureID:0 PayLoadSize:%d} data:%v}", aa, a, bb, b)
//	})
//}
//
//func TestQClient_DialQuic(t *testing.T) {
//	q := Initialize("127.0.0.1:2234")
//	var lines []*model.Line = []*model.Line{
//		{
//			Source:       "test1",
//			SourceIsCore: true,
//			SourceScene:  "test",
//			Target:       "test1",
//			TargetIsCore: true,
//			TargetScene:  "test",
//			Dependence:   "strong",
//			VisitCount:   10,
//		},
//	}
//	startTime := time.Now()
//	data, _ := proto.Marshal(&model.LineList{Messages: lines})
//	buf, _ := q.DialQuic(context.Background(), data)
//	endTime := time.Now()
//	elapsedTime := endTime.Sub(startTime)
//	fmt.Printf("cost %s Client: Got '%s'\n", elapsedTime, buf)
//}

func BenchmarkQClient_DialQuicRPC(b *testing.B) {
	q := Initialize("127.0.0.1:2234")
	var lines []*model.Line = []*model.Line{
		{
			Source:       "test1",
			SourceIsCore: true,
			SourceScene:  "test",
			Target:       "test1",
			TargetIsCore: true,
			TargetScene:  "test",
			Dependence:   "strong",
			VisitCount:   10,
		},
	}
	for i := 0; i < b.N; i++ {
		data, _ := proto.Marshal(&model.LineList{Messages: lines})
		_, _ = q.DialQuic(context.Background(), data)
	}
}

func TestQClient_DialQuic(t *testing.T) {
	q := Initialize("127.0.0.1:2234")
	var lines []*model.Line = []*model.Line{
		{
			Source:       "test1",
			SourceIsCore: true,
			SourceScene:  "test",
			Target:       "test1",
			TargetIsCore: true,
			TargetScene:  "test",
			Dependence:   "strong",
			VisitCount:   10,
		},
	}

	var mu sync.Mutex
	var counter int
	var timeP int64
	conc := make(chan struct{}, 9)
	timeout := 1 * time.Second

	go func() {
		for {
			conc <- struct{}{}
			go func() {
				defer func() {
					<-conc
				}()
				s := time.Now().UnixNano()
				data, _ := proto.Marshal(&model.LineList{Messages: lines})
				_, _ = q.DialQuic(context.Background(), data)
				e := time.Now().UnixNano()
				mu.Lock()
				timeP = timeP + e - s
				counter++
				mu.Unlock()
			}()
		}
	}()

	select {
	case <-time.After(timeout):
	}
	fmt.Printf("Total function executions: %d\n", counter)
	fmt.Printf("aveCost %d\n", timeP/int64(counter))
}
