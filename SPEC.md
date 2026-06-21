# ibot specification

`ibot` is one installable Go binary with two interfaces:

1. CLI commands capture desktop points, boxes, and images.
2. `ibot serve` hosts an embedded loopback-only web application that performs
   the same local capture workflows.

Both interfaces call the same Go generator and emit portable Python functions
that automate clicks with PyAutoGUI. Image bytes are embedded directly into
generated functions so no separate image file is required.
