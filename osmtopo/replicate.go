package osmtopo

import (
	"bufio"
	"net/http"
	"strconv"
	"strings"
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
