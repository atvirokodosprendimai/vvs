package database

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type writeRequest struct {
	fn     func(tx *gorm.DB) error
	result chan error
}

type WriteSerializer struct {
	db      *gorm.DB
	writeCh chan writeRequest
	done    chan struct{}
}

func NewWriteSerializer(db *gorm.DB) *WriteSerializer {
	ws := &WriteSerializer{
		db:      db,
		writeCh: make(chan writeRequest, 64),
		done:    make(chan struct{}),
	}
	go ws.run()
	return ws
}

func (ws *WriteSerializer) run() {
	defer close(ws.done)
	for req := range ws.writeCh {
		err := ws.db.Transaction(func(tx *gorm.DB) error {
			return req.fn(tx)
		})
		req.result <- err
	}
}

func (ws *WriteSerializer) Execute(ctx context.Context, fn func(tx *gorm.DB) error) error {
	req := writeRequest{fn: fn, result: make(chan error, 1)}
	select {
	case ws.writeCh <- req:
	case <-ctx.Done():
		return fmt.Errorf("write serializer: %w", ctx.Err())
	}
	select {
	case err := <-req.result:
		return err
	case <-ctx.Done():
		return fmt.Errorf("write serializer: %w", ctx.Err())
	}
}

func (ws *WriteSerializer) Close() {
	close(ws.writeCh)
	<-ws.done
}
