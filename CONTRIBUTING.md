# Contributing

The CLI and browser demo are simulations. Keep changes and examples explicit about which data and actions are synthetic.

## Go checks

```bash
go test -race ./...
go vet ./...
```

## Browser checks

Needs Go 1.23+, Node.js 24+ and npm. The build uses the repository's actual Go evaluator and controller. The browser tests serve the resulting local WebAssembly files; no substitute engine or cluster is used.

```bash
bash scripts/build-site.sh
npm ci --ignore-scripts
npx playwright install chromium firefox webkit
npm run test:site
SITE_BROWSER=firefox npm run test:site
SITE_BROWSER=webkit npm run test:site
```

The same build script assembles Pages and the PR test site. Checks cover mobile/desktop navigation, deferred engine downloads, keyboard access, automated contrast/accessibility rules, real gate scenarios, network retry and JavaScript-disabled instructions.
