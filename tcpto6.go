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

// Package tcpto6 provides an program that takes a net.Listener from systemd and accepts connections from it.
// For each accepted connection it dials to a predefined tcp6 address and bridges the connection if the dial succeeds.
package tcpto6

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"

	"dev.eqrx.net/rungroup"
	"github.com/coreos/go-systemd/v22/activation"
	"github.com/go-logr/logr"
)

// ToAddrEnvName is the name of the environment variable that contains the address that should be dialed for accepted
// connections. Must be in a format that net.Dial understands.
const ToAddrEnvName = "TCPV4TO6_DESTINATION_ADDR"

var (
	// errEnvMissing is internally raised if an env var is missing.
	errEnvMissing = errors.New("environment variable is not set")
	// errUnexpectedSocketAmount is internally raised if systemd passed more or less then 1 sockets to us.
	errUnexpectedSocketAmount = errors.New("systemd passed unexpected number of sockets")
)

// Run fetches the listening socket from systemd, the target address from the env var and calls handleListener
// with them. It closes the listener when the given context ctx is canceled.
//
// The source code repository contains the directory /init with an example .service and .socket file.
func Run(ctx context.Context, log logr.Logger) error {
	toAddr, ok := os.LookupEnv(ToAddrEnvName)
	if !ok {
		return fmt.Errorf("%w: %s", errEnvMissing, ToAddrEnvName)
	}

	listeners, err := activation.Listeners()
	if err != nil {
		return fmt.Errorf("could not get systemd sockets: %w", err)
	}

	if len(listeners) != 1 {
		return fmt.Errorf("%w: %v", errUnexpectedSocketAmount, listeners)
	}

	listener := listeners[0]

	group := rungroup.New(ctx)

	group.Go(func(context.Context) error { return handleListener(group, log, toAddr, listener) })

	// Close the listener when the group is asked to stop. This will cause the goroutine blocked in accept to return.
	group.Go(func(ctx context.Context) error {
		<-ctx.Done()
		if err := listener.Close(); err != nil {
			return fmt.Errorf("could not close listener: %w", err)
		}

		return nil
	})

	if err := group.Wait(); err != nil {
		return fmt.Errorf("listening group failed: %w", err)
	}

	return nil
}

// handleListener accepts from the given listener until it is closed. Closing the listener causes the method to return
// with nil. If accept returns any error other than net.ErrClosed error, it is returned. For each accepted
// connection a routine will be dispatched in the given rungroup group with NoCancelOnSuccess set and tasked
// to call handleConn.
func handleListener(group *rungroup.Group, log logr.Logger, toAddr string, l net.Listener) error {
	for {
		from, err := l.Accept()

		switch {
		case err == nil:
		case errors.Is(err, net.ErrClosed):
			return nil
		default:
			return fmt.Errorf("failed to accept new connection: %w", err)
		}

		group.Go(func(ctx context.Context) error {
			handleConn(ctx, log, from, toAddr)

			return nil
		}, rungroup.NoCancelOnSuccess)
	}
}

// handleConn tries to dial a tcp6 to the net.Dial compatible address toAddr once. If this succeeds, the given net.Conn
// from read and write channels get bridged to the write and read channels of the dialed connection respectively.
// Errors are logged using the logger log.
func handleConn(ctx context.Context, log logr.Logger, from net.Conn, dstAddr string) {
	to, err := (&net.Dialer{}).DialContext(ctx, "tcp6", dstAddr)
	if err != nil {
		log.Error(err, "couldn't connect to dstAddr. closing accepted connection")

		if err := from.Close(); err != nil {
			log.Error(err, "couldn't close accepted connection")
		}
	} else {
		bridgeStreams(ctx, log, to, from)
	}
}

// bridgeStreams copies all data between the streams to and from until an operations returns an error. This error is
// then logged and both interfaces are closed.
func bridgeStreams(ctx context.Context, log logr.Logger, to, from io.ReadWriteCloser) {
	group := rungroup.New(ctx)

	group.Go(func(context.Context) error {
		if _, err := io.Copy(to, from); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Error(err, "copy from->to failed")
		}

		return nil
	})
	group.Go(func(context.Context) error {
		if _, err := io.Copy(from, to); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Error(err, "copy from<-to failed")
		}

		return nil
	})
	group.Go(func(ctx context.Context) error {
		<-ctx.Done()

		if err := to.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Error(err, "could not close from stream")
		}

		return nil
	})
	group.Go(func(ctx context.Context) error {
		<-ctx.Done()

		if err := from.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Error(err, "could not close to stream")
		}

		return nil
	})

	if err := group.Wait(); err != nil {
		panic("did not expect errors")
	}
}
