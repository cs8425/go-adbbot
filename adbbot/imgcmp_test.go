// build for linux amd64: GOOS=linux GOARCH=amd64 go test -c -o test.x64
// build for linux arm7:  GOOS=linux GOARCH=arm GOARM=7 go test -c -o test.arm7
// run: ./test.x64 -test.bench=. -test.benchmem -test.cpu=1,2,4,8
/*
Intel(R) Core(TM) i7-3770 CPU @ 3.40GHz
Linux 4.10.0-42-generic #46~16.04.1-Ubuntu SMP Mon Dec 4 15:57:59 UTC 2017 x86_64 x86_64 x86_64 GNU/Linux
go version go1.10 linux/amd64
----
$./test.x64 -test.bench=. -test.benchmem -test.cpu=1,2,4,5,6,7,8,9,10,11,12
goos: linux
goarch: amd64
BenchmarkFind         	       1	2725483432 ns/op	      32 B/op	       3 allocs/op
BenchmarkFind-2       	       1	2670946926 ns/op	      32 B/op	       3 allocs/op
BenchmarkFind-4       	       1	2671332545 ns/op	      32 B/op	       3 allocs/op
BenchmarkFind-5       	       1	2670198873 ns/op	      32 B/op	       3 allocs/op
BenchmarkFind-6       	       1	2701776379 ns/op	      32 B/op	       3 allocs/op
BenchmarkFind-7       	       1	2670525606 ns/op	      32 B/op	       3 allocs/op
BenchmarkFind-8       	       1	2768968232 ns/op	      32 B/op	       3 allocs/op
BenchmarkFind-9       	       1	2669556739 ns/op	      32 B/op	       3 allocs/op
BenchmarkFind-10      	       1	2669626230 ns/op	      32 B/op	       3 allocs/op
BenchmarkFind-11      	       1	2755224084 ns/op	      32 B/op	       3 allocs/op
BenchmarkFind-12      	       1	2669283707 ns/op	      32 B/op	       3 allocs/op
BenchmarkFindP        	       2	2680034647 ns/op	     440 B/op	      12 allocs/op
BenchmarkFindP-2      	       1	1367356316 ns/op	     432 B/op	      12 allocs/op
BenchmarkFindP-4      	       2	 734510076 ns/op	    2032 B/op	      16 allocs/op
BenchmarkFindP-5      	       2	 713911670 ns/op	    1720 B/op	      16 allocs/op
BenchmarkFindP-6      	       2	 698560056 ns/op	     672 B/op	      14 allocs/op
BenchmarkFindP-7      	       2	 681689384 ns/op	    1552 B/op	      16 allocs/op
BenchmarkFindP-8      	       2	 669181356 ns/op	    2096 B/op	      16 allocs/op
BenchmarkFindP-9      	       2	 669365693 ns/op	    3584 B/op	      20 allocs/op
BenchmarkFindP-10     	       2	 671189713 ns/op	    2176 B/op	      17 allocs/op
BenchmarkFindP-11     	       2	 668917154 ns/op	    2000 B/op	      19 allocs/op
BenchmarkFindP-12     	       2	 668863885 ns/op	    1328 B/op	      16 allocs/op
BenchmarkFindP2       	       2	2677691696 ns/op	     624 B/op	      11 allocs/op
BenchmarkFindP2-2     	       1	1353310512 ns/op	     896 B/op	      14 allocs/op
BenchmarkFindP2-4     	       2	 708209410 ns/op	     696 B/op	      11 allocs/op
BenchmarkFindP2-5     	       2	 691183852 ns/op	     760 B/op	      12 allocs/op
BenchmarkFindP2-6     	       2	 671314304 ns/op	     904 B/op	      14 allocs/op
BenchmarkFindP2-7     	       2	 652309714 ns/op	     776 B/op	      12 allocs/op
BenchmarkFindP2-8     	       2	 636435353 ns/op	    3696 B/op	      18 allocs/op
BenchmarkFindP2-9     	       2	 636102590 ns/op	    2408 B/op	      15 allocs/op
BenchmarkFindP2-10    	       2	 636858353 ns/op	    4376 B/op	      21 allocs/op
BenchmarkFindP2-11    	       2	 636234124 ns/op	    1992 B/op	      17 allocs/op
BenchmarkFindP2-12    	       2	 642926611 ns/op	    1176 B/op	      13 allocs/op
PASS
----
remove FindP,
and rename FindP2 >> FindP
*/
package adbbot

import (
	"image"
	"testing"
)

var tmpl image.Image
var input image.Image

func init() {
	input, _ = OpenImage("testdata/input.png")
	tmpl, _ = OpenImage("testdata/tmpl.png")
}

func BenchmarkFind(b *testing.B) {
	for i := 0; i < b.N; i++ {
		x, y, _ := Find(input, tmpl)
		if x != 464 || y != 694 {
			b.Fatalf("sub image find result failed = %v %v", x, y)
		}
	}
}

func BenchmarkFindP(b *testing.B) {
	for i := 0; i < b.N; i++ {
		x, y, _ := FindP(input, tmpl)
		if x != 464 || y != 694 {
			b.Fatalf("sub image find result failed = %v %v", x, y)
		}
	}
}


