package adbbot

import (
	"encoding/binary"
	"errors"
	"io"

	// "fmt"

	"compress/flate"
	"net"
	"time"

	"github.com/golang/snappy"
)

const xorTag byte = 0x00

var VTagMaxSize uint64 = 128 * 1024 * 1024 // 128MB, should enough for 4k raw image

var ErrLengthOutOfRange = errors.New("Length Out of Range")

func ReadTagByte(conn io.Reader) ([]byte, error) {
	buf := make([]byte, 1, 256)
	_, err := conn.Read(buf[:1])
	if err != nil {
		return nil, err
	}

	taglen := int(buf[0] ^ xorTag)
	n, err := io.ReadFull(conn, buf[:taglen])
	if err != nil {
		return nil, err
	}

	// fmt.Println("ReadTag:", taglen, buf[:n], string(buf[:n]))

	return buf[:n], nil
}

func WriteTagByte(conn io.Writer, tag []byte) (err error) {
	n := len(tag)
	if n > 255 {
		return ErrLengthOutOfRange
	}

	buf := make([]byte, 0, n+1)
	buf = append(buf, byte(n)^xorTag)
	buf = append(buf, tag...)

	// fmt.Println("WriteTag:", n, buf[:n+1], []byte(tag))

	_, err = conn.Write(buf[:n+1])
	return
}

func ReadTagStr(conn io.Reader) (string, error) {
	buf, err := ReadTagByte(conn)
	return string(buf), err
}

func WriteTagStr(conn io.Writer, tag string) (err error) {
	return WriteTagByte(conn, []byte(tag))
}

type byteReader struct {
	io.Reader
}

func (b *byteReader) ReadByte() (byte, error) {
	buf := make([]byte, 1, 1)
	_, err := b.Read(buf)
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}

func ReadVTagByte(conn io.Reader) ([]byte, error) {
	reader := &byteReader{conn}
	taglen, err := binary.ReadUvarint(reader)
	if err != nil {
		return nil, err
	}

	if taglen > VTagMaxSize {
		return nil, ErrLengthOutOfRange
	}

	buf := make([]byte, 0, taglen)
	n, err := io.ReadFull(conn, buf[:taglen])
	if err != nil {
		return nil, err
	}

	// fmt.Println("ReadVTag:", taglen, buf[:n], string(buf[:n]))
	// fmt.Println("ReadVTag:", taglen, n)

	return buf[:n], nil
}

func WriteVTagByte(conn io.Writer, tag []byte) (err error) {
	n := len(tag)

	if uint64(n) > VTagMaxSize {
		return ErrLengthOutOfRange
	}

	over := make([]byte, 10, 10)
	overlen := binary.PutUvarint(over, uint64(n))

	buf := make([]byte, 0, n+overlen)
	buf = append(buf, over[:overlen]...)
	buf = append(buf, tag...)

	// fmt.Println("WriteVTag:", n, overlen, buf, []byte(tag))

	_, err = conn.Write(buf)
	return
}

func ReadVLen(conn io.Reader) (int64, error) {
	reader := &byteReader{conn}
	return binary.ReadVarint(reader)
}

func WriteVLen(conn io.Writer, n int64) (err error) {
	over := make([]byte, 10, 10)
	overlen := binary.PutVarint(over, int64(n))
	_, err = conn.Write(over[:overlen])
	return
}

type CompStream struct {
	Conn net.Conn
	w    *snappy.Writer
	r    *snappy.Reader
}

func (c *CompStream) Read(p []byte) (n int, err error) {
	return c.r.Read(p)
}

func (c *CompStream) Write(p []byte) (n int, err error) {
	n, err = c.w.Write(p)
	err = c.w.Flush()
	return n, err
}

func (c *CompStream) Close() error {
	return c.Conn.Close()
}

// LocalAddr satisfies net.Conn interface
func (c *CompStream) LocalAddr() net.Addr {
	if ts, ok := c.Conn.(interface {
		LocalAddr() net.Addr
	}); ok {
		return ts.LocalAddr()
	}
	return nil
}

// RemoteAddr satisfies net.Conn interface
func (c *CompStream) RemoteAddr() net.Addr {
	if ts, ok := c.Conn.(interface {
		RemoteAddr() net.Addr
	}); ok {
		return ts.RemoteAddr()
	}
	return nil
}

func (c *CompStream) SetReadDeadline(t time.Time) error {
	return c.Conn.SetReadDeadline(t)
}

func (c *CompStream) SetWriteDeadline(t time.Time) error {
	return c.Conn.SetWriteDeadline(t)
}

func (c *CompStream) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	if err := c.SetWriteDeadline(t); err != nil {
		return err
	}
	return nil
}

func NewCompStream(conn net.Conn, level int) *CompStream {
	c := new(CompStream)
	c.Conn = conn
	c.w = snappy.NewBufferedWriter(conn)
	c.r = snappy.NewReader(conn)
	return c
}

type FlateStream struct {
	Conn net.Conn
	w    *flate.Writer
	r    io.ReadCloser
}

func (c *FlateStream) Read(p []byte) (n int, err error) {
	return c.r.Read(p)
}

func (c *FlateStream) Write(p []byte) (n int, err error) {
	n, err = c.w.Write(p)
	err = c.w.Flush()
	return n, err
}

func (c *FlateStream) Close() error {
	return c.Conn.Close()
}

// LocalAddr satisfies net.Conn interface
func (c *FlateStream) LocalAddr() net.Addr {
	if ts, ok := c.Conn.(interface {
		LocalAddr() net.Addr
	}); ok {
		return ts.LocalAddr()
	}
	return nil
}

// RemoteAddr satisfies net.Conn interface
func (c *FlateStream) RemoteAddr() net.Addr {
	if ts, ok := c.Conn.(interface {
		RemoteAddr() net.Addr
	}); ok {
		return ts.RemoteAddr()
	}
	return nil
}

func (c *FlateStream) SetReadDeadline(t time.Time) error {
	return c.Conn.SetReadDeadline(t)
}

func (c *FlateStream) SetWriteDeadline(t time.Time) error {
	return c.Conn.SetWriteDeadline(t)
}

func (c *FlateStream) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	if err := c.SetWriteDeadline(t); err != nil {
		return err
	}
	return nil
}

func NewFlateStream(conn net.Conn, level int) *FlateStream {
	c := new(FlateStream)
	c.Conn = conn
	c.w, _ = flate.NewWriter(conn, level)
	c.r = flate.NewReader(conn)
	return c
}
