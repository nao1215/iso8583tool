# Security Policy

## Reporting a Vulnerability

If you discover any security-related issues or vulnerabilities, please contact us at [n.chika156@gmail.com](mailto:n.chika156@gmail.com). We appreciate your responsible disclosure and will work with you to address the issue promptly.

## Handling of Cardholder Data

`iso8583tool` is a debugging aid, not a vault. Its defaults are fail-safe:

- `view`, `diff`, and `redact` mask cardholder data by default — the PAN (kept
  to BIN + last four), full track and PIN data, the EMV tags that carry them,
  unknown TLV tags, and a PAN embedded in a free-form private field.
- Raw values are available only by explicit opt-in: `view --unsafe` and
  `diff --unsafe`. `redact` has no raw mode by design. Use `--unsafe` only on a
  trusted machine for fault analysis; never paste its output into a ticket,
  chat, or log.
- `convert` is the exception: it emits the document **unmasked** so messages
  round-trip. Treat `convert` JSON output as sensitive.

This is not a substitute for PCI DSS controls. Full track data and PIN blocks
are sensitive authentication data and must never be stored after authorization.

## Supported Versions

We recommend using the latest release for the most up-to-date and secure experience. Security updates are provided for the latest stable version.

## Security Policy

- Security issues are treated with the highest priority.
- We follow responsible disclosure practices.
- Fixes for security vulnerabilities will be provided in a timely manner.

## Acknowledgments

We would like to thank the security researchers and contributors who responsibly report security issues and work with us to make our project more secure.

Thank you for your help in making our project safe and secure for everyone.
