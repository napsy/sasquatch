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
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
)

var clientTimeout = 10 * time.Second

func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, clientTimeout)
}

func runHealthCheck(name string, svc *service) {
	var (
		b   []byte
		err error
	)
	for {
		svc.HealthCheck.markHealthy()
		start := time.Now()
		switch svc.HealthCheck.Type {
		case "http":
			transport := http.Transport{
				Dial: dialTimeout,
			}
			client := http.Client{
				Transport: &transport,
			}
			response, err := client.Get(svc.HealthCheck.Location)
			if err != nil {
				svc.HealthCheck.markUnhealthy()
			} else {
				b, err = ioutil.ReadAll(response.Body)
				if err != nil {
					svc.HealthCheck.markUnhealthy()
				}
			}
		case "script":
			b, err = exec.Command(svc.HealthCheck.Location, name).CombinedOutput()
			if err != nil {
				fmt.Printf("Couldn't execute check script: %v\n", err)
				svc.HealthCheck.markUnhealthy()
			}
		}
		d := time.Now().Sub(start)
		// Check error only if the health check endpoint was successful
		if err == nil && svc.HealthCheck.Error == "message" {
			tokens := strings.SplitN(svc.HealthCheck.Value, " ", 2)
			operator, value := tokens[0], tokens[1]
			value = strings.Replace(value, `"`, "", -1)
			switch operator {
			case "!=":
				if !strings.Contains(string(b), value) {
					svc.HealthCheck.markUnhealthy()
				}
			case "==":
				if strings.Contains(string(b), value) {
					svc.HealthCheck.markUnhealthy()
				}
			}
		}
		if svc.HealthCheck.unhealthy {
			// Run response script
			if svc.HealthCheck.Response != "" {
				out, err := exec.Command(svc.HealthCheck.Response, name).CombinedOutput()
				if err != nil {
					fmt.Printf("Couldn't execute script: %v\n", err)
				}
				fmt.Printf("%v", string(out))
			}

		}
		svc.db.Create(&Service{
			Name:      name,
			Timestamp: time.Now().Unix(),
			Interval:  int(d),
			Failed:    svc.HealthCheck.unhealthy,
		})
		time.Sleep(time.Duration(svc.HealthCheck.Interval) * time.Second)
	}
}

func getAvailability(name string, from, to time.Time, db *gorm.DB) float64 {
	s := []Service{}
	db.Where("created_at BETWEEN ? AND ? AND Name = ?", from, to, name).Find(&s)
	failed := 0
	n := len(s)
	for i := 0; i < n; i++ {
		if !s[i].Failed {
			continue
		}
		failed++
	}
	return (float64(n-failed) / float64(n)) * 100
}
