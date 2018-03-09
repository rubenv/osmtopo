package osmtopo

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/cheggaaa/pb"
)

func downloadProgress(tmpName, url string) (string, string, error) {
	tmp, err := ioutil.TempFile("", tmpName)
	if err != nil {
		return "", "", err
	}
	defer tmp.Close()

	resp, err := http.Get(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
	}

	bar := pb.New(int(resp.ContentLength)).SetUnits(pb.U_BYTES)
	bar.Start()
	defer bar.Finish()

	reader := bar.NewProxyReader(resp.Body)

	h := md5.New()
	w := io.MultiWriter(tmp, h)

	_, err = io.Copy(w, reader)
	if err != nil {
		return "", "", err
	}

	return tmp.Name(), fmt.Sprintf("%02x", h.Sum(nil)), err
}
