/*
Copyright (c) 2018 Visionect

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/
package main

import (
	"flag"
	"io/ioutil"
	"sync"
	"time"

	"github.com/jinzhu/gorm"
	yaml "gopkg.in/yaml.v2"
)

type healthCheck struct {
	Type     string
	Location string
	Error    string
	Value    string
	Response string
	Retries  int
	Interval int

	l             sync.RWMutex
	unhealthy     bool
	unhealthyDate time.Time
	retryCount    int
}

func (health healthCheck) Unhealthy() bool {
	health.l.RLock()
	defer health.l.RUnlock()
	return health.unhealthy
}

func (health healthCheck) UnhealthyDate() string {
	health.l.RLock()
	defer health.l.RUnlock()
	return health.unhealthyDate.Format("2006-02-01 15:04:05.999")
}

func (health *healthCheck) markUnhealthy() {
	health.l.Lock()
	health.retryCount++
	if health.Retries > 0 && health.retryCount <= health.Retries {
		health.l.Unlock()
		return
	}
	health.unhealthy = true
	health.unhealthyDate = time.Now()
	health.retryCount = 0
	health.l.Unlock()
}

func (health *healthCheck) markHealthy() {
	health.l.Lock()
	health.unhealthy = false
	health.l.Unlock()
}

type service struct {
	l           sync.Mutex
	HealthCheck healthCheck
	Tags        []string
	db          *gorm.DB
}

func (svc *service) GetAvailability(name string) float64 {
	to := time.Now()
	from := to.AddDate(0, -1, 0)

	// Get one month old data
	return getAvailability(name, from, to, svc.db)
}

type cfg struct {
	Database string
	Services map[string]*service
}

var (
	endChan = make(chan bool)
)

func main() {
	c := cfg{}

	flagCfg := flag.String("c", "config.yaml", "configuration file")
	flagHttp := flag.Bool("http", true, "run the http server")
	flag.Parse()
	o, err := ioutil.ReadFile(*flagCfg)
	if err != nil {
		panic(err)
	}
	if err := yaml.Unmarshal(o, &c); err != nil {
		panic(err)
	}

	db, err := initDB(c.Database)
	if err != nil {
		panic(err)
	}

	for k, v := range c.Services {
		v.db = db
		go runHealthCheck(k, v)
	}
	if *flagHttp {
		go startWeb(&c)
	}
	<-endChan
}
