package main

import (
	"bufio"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"os"
	"sync"
)

// var (
// source = rand.NewSource(time.Now().UnixNano())
// randm  = rand.New(source)
// )

type ListManager struct {
	mu             sync.Mutex
	index          int
	items          []*ListItem
	leases         map[string]string
	leasesByTaskId map[string]string
	Context        context.Context
	WaitGroup      sync.WaitGroup
}

type ListItem struct {
	hash string
	line string
}

func NewListManager() *ListManager {
	lm := new(ListManager)
	lm.Context = context.Background()
	lm.leases = make(map[string]string)
	lm.leasesByTaskId = make(map[string]string)
	lm.index = 0
	return lm
}

func (lm *ListManager) Read(filename string) error {
	file, err := os.Open(filename)

	if err != nil {
		return err
	}

	fileScanner := bufio.NewScanner(file)
	fileScanner.Split(bufio.ScanLines)

	for fileScanner.Scan() {
		line := fileScanner.Text()
		lm.AddLine(line)
	}

	return nil
}

func (lm *ListManager) Count() int {
	return len(lm.items)
}

func (lm *ListManager) AddLine(line string) *ListItem {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	li := &ListItem{line: line}

	li.hash = fmt.Sprintf("%x", md5.Sum([]byte(line)))

	lm.items = append(lm.items, li)

	return li
}

func (lm *ListManager) AddLines(lines ...string) {
	for _, line := range lines {
		lm.AddLine(line)
	}
}

func (lm *ListManager) unlease(taskId string) {
	pHash := lm.leasesByTaskId[taskId]

	delete(lm.leases, pHash)
	delete(lm.leasesByTaskId, taskId)
}

func (lm *ListManager) Lease(taskId string) (*ListItem, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.unlease(taskId)

	leased := false
	attempts := 0

	var item *ListItem
	var err error

	for !leased {

		i := lm.index
		lm.index = i + 1
		attempts = attempts + 1

		if lm.index == len(lm.items) {
			lm.index = 0
		}

		li := lm.items[i]

		if _, ok := lm.leases[li.hash]; !ok {
			lm.leasesByTaskId[taskId] = li.hash
			lm.leases[li.hash] = taskId

			item = li
			leased = true
		}

		if attempts == 5 {
			err = errors.New("unable to find an unleased item")
			break
		}

	}

	return item, err

}
