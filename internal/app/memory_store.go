package app

import "context"

type MemoryStore struct {
	tasks []Task
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tasks: []Task{
			{ID: 1, Title: "学习 Codex 本地开发", Done: true},
			{ID: 2, Title: "用 Git 管理版本", Done: false},
			{ID: 3, Title: "部署到腾讯云 Lighthouse", Done: false},
		},
	}
}

func (s *MemoryStore) Name() string {
	return "memory"
}

func (s *MemoryStore) Ping(ctx context.Context) error {
	return nil
}

func (s *MemoryStore) ListTasks(ctx context.Context) ([]Task, error) {
	tasks := make([]Task, len(s.tasks))
	copy(tasks, s.tasks)
	return tasks, nil
}
