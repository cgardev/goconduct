# Security policy

`goconduct` is experimental alpha software and has no supported release series yet.

Report a suspected vulnerability through GitHub's private security reporting feature when it is available.
Do not publish exploit details in a public issue before a fix exists.

The dashboard binds to `127.0.0.1` by default.
Changing the address can expose source paths and can permit quality-tool execution through Connect RPC.
Place any remotely reachable instance behind authentication and network access controls.

Quality plugins execute configured local tools without a command shell.
Treat `.goconduct.json` as executable build configuration and review changes to command paths carefully.
