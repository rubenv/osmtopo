package osmtopo

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/northbright/ctx/ctxdownload"
)

func fetchLatestSequence(url string) (int64, error) {
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	url += "state.txt"

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	pre := "sequenceNumber="
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, pre) {
			seq, err := strconv.ParseInt(line[len(pre):], 10, 64)
			if err != nil {
				return 0, err
			}
			return seq, nil
		}
	}

	return 0, nil
}

func fetchChangeset(ctx context.Context, url string, seq int64, folder string) (string, error) {
	url = changesetUrl(url, seq)
	buf := make([]byte, 2*1024*1024)
	return ctxdownload.Download(ctx, url, folder, "", buf, 24*3600)
}

func changesetUrl(url string, seq int64) string {
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	url += fmt.Sprintf("%03d/%03d/%03d.osc.gz", seq/1e6, seq/1e3%1e3, seq%1e3)
	return url
}
