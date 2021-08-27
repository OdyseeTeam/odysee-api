package olapdb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lbryio/lbrytv/apps/watchman/gen/reporter"
	"github.com/lbryio/lbrytv/apps/watchman/log"
	"github.com/pkg/errors"
)

type BatchWriter struct {
	width, capacity int
	interval        time.Duration

	batch    [][]interface{}
	rcvChan  chan []interface{}
	stopChan chan bool
}

func NewBatchWriter(interval time.Duration, width int) *BatchWriter {
	capacity := 100000
	b := BatchWriter{
		width:    width,
		interval: interval,
		capacity: capacity,
		batch:    [][]interface{}{},
		rcvChan:  make(chan []interface{}, capacity*3),
		stopChan: make(chan bool, 1),
	}
	return &b
}

func (b *BatchWriter) Start() {
	var counter int
	var stop bool
	ticker := time.NewTicker(b.interval)

	for !stop {
		select {
		case el := <-b.rcvChan:
			if el == nil {
				continue
			}
			log.Log.Debugw("batch element received", "el", el, "count", counter)
			b.batch = append(b.batch, el)
			counter++
		case <-ticker.C:
			if counter == 0 {
				log.Log.Info("no elements in the current batch")
				continue
			}
			log.Log.Infow("preparing a batch write", "count", counter)
			err := b.writeBatch()
			if err != nil {
				log.Log.Warnw("could not write a batch", "err", err)
			} else {
				log.Log.Infow("batch written", "count", counter)
			}
			counter = 0
		}
	}

	// Write out the remaining records after Stop() has been called.
	if counter > 0 {
		err := b.writeBatch()
		if err != nil {
			log.Log.Warnw("could not write the last batch", "err", err)
		}
	}
	b.stopChan <- true
}

func (b *BatchWriter) Stop() {
	close(b.rcvChan)
}

func (b *BatchWriter) Write(r *reporter.PlaybackReport, addr string, ts string) error {
	args, err := prepareArgs(r, addr, ts)
	if err != nil {
		return err
	}
	b.rcvChan <- args
	return nil
}

func (b *BatchWriter) writeBatch() error {
	ph := fmt.Sprintf("(%s)", "?"+strings.Repeat(", ?", b.width-1))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "cannot begin")
	}
	q := prepareInsertQuery(ph)
	stmt, err := tx.Prepare(q)
	if err != nil {
		return errors.Wrap(err, "cannot prepare")
	}
	log.Log.Debugw("insert query prepared", "q", q)
	defer stmt.Close()

	for i, row := range b.batch {
		_, err := stmt.Exec(row...)
		if err != nil {
			return err
		}
		log.Log.Debugw("row written", "row", row, "num", i)
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "cannot commit")
	}

	b.batch = [][]interface{}{}
	return nil
}

// func (b *BatchWriter) writeBatch(num int) error {
// 	valueStrings := make([]string, 0, num)
// 	valueArgs := make([]interface{}, 0, num*b.width)
// 	ph := fmt.Sprintf("(%s)", "?"+strings.Repeat(", ?", b.width-1))

// 	for _, row := range b.batch[:num] {
// 		valueStrings = append(valueStrings, ph)
// 		valueArgs = append(valueArgs, row...)
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
// 	defer cancel()
// 	tx, err := conn.BeginTx(ctx, nil)
// 	if err != nil {
// 		return errors.Wrap(err, "cannot begin")
// 	}
// 	q := prepareInsertQuery(strings.Join(valueStrings, ", "))
// 	log.Log.Debugw("insert query prepared", "q", q)
// 	stmt, err := tx.Prepare(q)
// 	if err != nil {
// 		return errors.Wrap(err, "cannot prepare")
// 	}
// 	defer stmt.Close()
// 	res, err := stmt.Exec(valueArgs...)

// 	if err != nil {
// 		return err
// 	}

// 	if err := tx.Commit(); err != nil {
// 		return errors.Wrap(err, "cannot commit")
// 	}

// 	log.Log.Infow("batch written", "number", num, "result", res)
// 	return nil
// }
