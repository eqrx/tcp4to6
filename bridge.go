// Copyright (C) 2021 Alexander Sowitzki
//
// This program is free software: you can redistribute it and/or modify it under the terms of the
// GNU Affero General Public License as published by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied
// warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License for more
// details.
//
// You should have received a copy of the GNU Affero General Public License along with this program.
// If not, see <https://www.gnu.org/licenses/>.

package tcp4to6

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/eqrx/rungroup"
)

// BridgeStreams connects two io.ReadWriteCloser by copying all data received from their write channels to the read
// channel of partner. This method closes both streams and returns regardless if data is in the process written when
// wither the given context ctx is canceled, one of the streams returns io.EOF, os.ErrClosed or net.ErrClosed from
// the Read or Write method or when an unexpected error occurs.
//
// The canceling of the context or the capture of mentioned errors causes this method to return nil as error.
// If an unexpected errors occurs while copying or are returned from the streams Close method, a descriptive error
// is returned for all of them.
func BridgeStreams(ctx context.Context, a, b io.ReadWriteCloser) error {
	group := rungroup.New(ctx)

	group.Go(func(context.Context) error { return copyStream(a, b) })
	group.Go(func(context.Context) error { return copyStream(b, a) })
	group.Go(func(ctx context.Context) error { return closeAfterDone(ctx, a, b) })

	errs := group.Wait()
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		//nolint: goerr113 // Slice of errors, no wrapping possible or useful.
		return fmt.Errorf("multiple errors while bridging streams: %v", errs)
	}
}

// copyStream uses io.Copy to copy the contexts of io.Reader src to io.Writer dst.
// If io.Copy returns nil, os.ErrClosed or net.ErrClosed this method returns nil.
// Unexpected errors are returned.
func copyStream(dst io.Writer, src io.Reader) error {
	_, err := io.Copy(dst, src)

	switch {
	case err == nil || errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed):
		return nil
	default:
		return fmt.Errorf("io.Copy failed: %w", err)
	}
}

// closeAfterDone waits for the given context ctx to be done and closes the io.Closer set closers afterwards. If no
// closer returns an error this methods returns nil as well. If exactly one closer returns an error this error is
// returned directly. If multiple closers return errors the method returns a descriptive error for all of them.
func closeAfterDone(ctx context.Context, closers ...io.Closer) error {
	<-ctx.Done()

	errs := make([]error, 0, len(closers))

	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		//nolint: goerr113 // Slice of errors, no wrapping possible or useful.
		return fmt.Errorf("multiple errors while closing closers: %v", errs)
	}
}
