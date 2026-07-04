package queue

import (
	"context"
	"log"

	"github.com/hibiken/asynq"
)

// QueueServer Asynq Worker服务器
type QueueServer struct {
	server *asynq.Server
	mux    *asynq.ServeMux
}

// NewQueueServer 创建Worker服务器
func NewQueueServer(redisAddr, password string, db, concurrency int) *QueueServer {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     redisAddr,
			Password: password,
			DB:       db,
		},
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				log.Printf("[ERROR] queue: task %s failed: %v", task.Type(), err)
			}),
		},
	)
	mux := asynq.NewServeMux()
	return &QueueServer{server: srv, mux: mux}
}

// HandleFunc 注册任务处理函数
func (s *QueueServer) HandleFunc(pattern string, handler func(context.Context, *asynq.Task) error) {
	s.mux.HandleFunc(pattern, handler)
}

// Run 启动Worker
func (s *QueueServer) Run() error {
	return s.server.Run(s.mux)
}
