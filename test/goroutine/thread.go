package main

import (
	"fmt"
	"sync"
	"time"
)

var n int
var wg sync.WaitGroup
var networkNodes []Elastico

type Elastico struct {
	commID int
}

func (e *Elastico) ElasticoInit() {
	e.commID = -1
}

func execute(index int) {
	defer wg.Done()
	if index == 49 {
		time.Sleep(100 * time.Millisecond)
		for i := 0; i < n; i++ {
			fmt.Println("commid for index -", i, "--", networkNodes[index].commID)
		}
	} else {
		networkNodes[index].commID = index
	}

}

func createNodes() {
	networkNodes = make([]Elastico, n)
	for i := 0; i < n; i++ {
		networkNodes[i].ElasticoInit()
	}
}

func main() {
	n = 50
	wg.Add(int(n))
	createNodes()
	for i := 0; i < n; i++ {
		go execute(i)
	}
	wg.Wait()
	for i := 0; i < n; i++ {
		fmt.Println("index- ", i, networkNodes[i].commID)
	}
}
