[![Go Reference](https://pkg.go.dev/badge/github.com/eqrx/tcp4to6.svg)](https://pkg.go.dev/github.com/eqrx/tcp4to6)
# tcp4to6

Listen on a tcp4 address passed by systemd and forward connections to a specified tcp6 address. Except for the systemd
socket path this could be done with tools like `socat` but I want to have something that can do exactly that and 
nothing more.

This project is released under GNU Affero General Public License v3.0, see LICENCE file in this repo for more info.
