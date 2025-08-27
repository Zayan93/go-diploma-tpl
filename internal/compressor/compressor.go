package compressor

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/Zayan93/go-diploma-tpl/internal/logger"
)

// создаем новый Writer, с добавлением функциональности компрессии
type compressWriter struct {
	w             http.ResponseWriter
	zw            *gzip.Writer
	statusCode    int
	started       bool
	shouldZip     bool
	headerWritten bool
}

func (c *compressWriter) Header() http.Header {
	return c.w.Header()
}

func (c *compressWriter) WriteHeader(statusCode int) {
	if c.headerWritten {
		return
	}
	c.statusCode = statusCode
	c.headerWritten = true
	// Не пишем пока в оригинальный w — отложим до Write()
}

func (c *compressWriter) Write(p []byte) (int, error) {
	if !c.started {
		c.started = true

		// По Content-Type решаем, надо ли сжимать
		ct := c.w.Header().Get("Content-Type")
		if strings.HasPrefix(ct, "application/json") || strings.HasPrefix(ct, "text/html") {
			c.shouldZip = true
			c.zw = gzip.NewWriter(c.w)
			c.w.Header().Set("Content-Encoding", "gzip")
		}

		// Отправим отложенный статус
		if c.statusCode != 0 {
			c.w.WriteHeader(c.statusCode)
		} else {
			c.w.WriteHeader(http.StatusOK)
		}
	}

	if c.shouldZip {
		return c.zw.Write(p)
	}
	return c.w.Write(p)
}

func (c *compressWriter) Close() error {
	if c.shouldZip && c.zw != nil {
		return c.zw.Close()
	}
	return nil
}

// newCompressWriter основной интерфейс который служит заменой ResponseWriter
func NewCompressWriter(w http.ResponseWriter) *compressWriter {
	return &compressWriter{
		w:  w,
		zw: gzip.NewWriter(w),
	}
}

// compressReader реализует интерфейс io.ReadCloser и позволяет прозрачно для сервера
// декомпрессировать получаемые от клиента данные
type compressReader struct {
	r  io.ReadCloser
	zr *gzip.Reader
}

func (c compressReader) Read(p []byte) (n int, err error) {
	return c.zr.Read(p)
}

func (c *compressReader) Close() error {
	if err := c.r.Close(); err != nil {
		return err
	}
	return c.zr.Close()
}

func NewCompressReader(r io.ReadCloser) (*compressReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &compressReader{
		r:  r,
		zr: zr,
	}, nil
}

func GzipMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Декодируем входящий gzip, если есть
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			cr, err := NewCompressReader(r.Body)
			if err != nil {
				http.Error(w, "Bad Request - invalid gzip encoding", http.StatusBadRequest)
				return
			}
			r.Body = cr
			defer cr.Close()
			logger.Log.Info("Входящий gzip-декодирован")
		}

		// Проверяем, хочет ли клиент gzip
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			cw := NewCompressWriter(w)
			defer cw.Close()
			logger.Log.Info("Исходящий gzip будет использоваться")
			h.ServeHTTP(cw, r)
			return
		}

		// Без компрессии
		h.ServeHTTP(w, r)
	})
}
