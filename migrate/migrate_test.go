package migrate_test

import (
	"fmt"
	"github.com/mylxsw/go-utils/must"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestCleanSQL(t *testing.T) {
	data := string(must.Must(os.ReadFile("/Users/mylxsw/Downloads/data.sql")))
	for _, line := range strings.Split(data, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fmt.Println(strconv.Quote(line) + ",")
	}
}
