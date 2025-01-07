# egr

**egr** is a small Go package that extends [errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup) with a typed channel, allowing you to push items into a queue and process them in concurrent goroutines with error propagation. The goal of the package is to provide a standard way to use errgroup, while maintaining most of its flexibility.

## Installation

```console
go get github.com/invisiblefunnel/egr
```

## Example

```go
package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/invisiblefunnel/egr"
)

// Let's rewrite the pipeline example from the errgroup docs.
// https://pkg.go.dev/golang.org/x/sync@v0.10.0/errgroup#example-Group-Pipeline
func main() {
	m, err := MD5All(context.Background(), ".")
	if err != nil {
		log.Fatal(err)
	}

	for k, sum := range m {
		fmt.Printf("%s:\t%x\n", k, sum)
	}
}

type result struct {
	path string
	sum  [md5.Size]byte
}

// MD5All reads all the files in the file tree rooted at root and returns a map
// from file path to the MD5 sum of the file's contents. If the directory walk
// fails or any read operation fails, MD5All returns an error.
func MD5All(ctx context.Context, root string) (map[string][md5.Size]byte, error) {
	queueSize := 1 // choose based on workload
	numDigesters := 20

	digesters, ctx := egr.WithContext[string](ctx, queueSize)
	collector, ctx := egr.WithContext[result](ctx, queueSize)
	collector.SetLimit(1)

	for i := 0; i < numDigesters; i++ {
		digesters.Go(func(queue <-chan string) error {
			for path := range queue {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				err = collector.Push(ctx, result{path, md5.Sum(data)})
				if err != nil {
					return err
				}
			}
			return nil
		})
	}

	m := make(map[string][md5.Size]byte)

	// Only launch one collector goroutine to update the map
	collector.Go(func(queue <-chan result) error {
		for r := range queue {
			m[r.path] = r.sum
		}
		return nil
	})

	// Walk the directory and Push paths to the digesters queue
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return digesters.Push(ctx, path)
	})
	if err != nil {
		return nil, err
	}

	// Wait for digesters to finish
	if err := digesters.Wait(); err != nil {
		return nil, err
	}

	// Wait for collector to finish
	if err := collector.Wait(); err != nil {
		return nil, err
	}

	return m, nil
}
```

## License

This project is licensed under the BSD 3-Clause License (to match errgroup's license).
