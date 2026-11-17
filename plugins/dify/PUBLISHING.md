# Marketplace publishing checklist

This file prepares a later pull request to https://github.com/langgenius/dify-plugins. This repository change does not open that pull request, does not create a publisher account, and does not submit the package.

Fetched on 2026-11-17:

- Submission guide: https://docs.dify.ai/en/develop-plugin/publishing/marketplace-listing/submit-plugin-to-marketplace
- Live template: https://github.com/langgenius/dify-plugins/blob/main/.github/pull_request_template.md
- Requirements: https://github.com/langgenius/dify-plugins/blob/main/docs/plugin-submission-requirements.md

Re-read those pages before submitting. The template is part of the review contract.

## Suggested template values

These match this package. They are not a submitted form.

- Author: `openanysi`
- Plugin name: `openany`
- Version: `0.1.0`
- Source repository: `https://github.com/openanysi/openany-mcp` (plugin directory `plugins/dify`)
- Contact: `team@openany.si`
- Submission type: New plugin
- Risk level: Medium

Medium is the honest box. The plugin calls one fixed documented HTTPS API, which is the Low pattern, and it also sends user-supplied `input` to OpenAny. `run_api` can perform a write or other side effect on an upstream API, and a result can contain personal or business data the selected API returns. The higher box is Medium. It is not High: the plugin does not execute commands, SQL, or browser automation, and it does not fetch a user-chosen URL.

Check exactly one risk box. The marketplace workflow looks for a single line `- [x] Medium risk`.

## Security and privacy notes for the marketplace PR

- No command execution, SQL, SSH, filesystem access, or browser automation.
- No arbitrary URL fetching. The endpoint is fixed at `https://mcp.openany.si`. Redirects are refused. Proxies are disabled.
- The API key is a Dify secret field, not a tool parameter. It is sent only on `run_api`.
- Credential validation does not call OpenAny.
- Timeouts: 30 seconds for catalog handshake and catalog tools, 120 seconds for `run_api`. A timed-out `run_api` is not retried.
- Errors redact the key.
- Privacy notice: `PRIVACY.md`, which links to https://openany.si/privacy/.

## Local validation to paste

Replace the commands with the output from the machine that builds the package. Record the CLI version, the SDK version, the unittest count, the package filename and size, `unzip -t`, and a file list that shows tests, `.env`, and caches are absent.

This change's results belong in the OpenAny pull request, not in a marketplace pull request, until a person submits it.

A free `openany_overview` smoke on 2026-11-17 returned `api_count=3278` and `model_count=153` (chat/reasoning 67, image 40, video 43, audio 3). The manifest keeps the approved marketing figures 3,200+ and 168+. Those are not the live totals. Agents should read `openany_overview`.

The marketplace validator at `langgenius/dify-marketplace-toolkit` `skill/local-difypkg-validator/validator/validate-difypkg.py` (the path named in the submission doc, `validator/validate-difypkg.py`, is not at the toolkit root in the clone from 2026-11-17) reported 0 blocking failures. Warnings to keep in the marketplace PR:

- `meta.minimum_dify_version` is absent on purpose. It stays absent until a Dify runtime install shows the oldest working version.
- `openany.si` is the key-creation URL in error text. The plugin does not open a connection to it. The only request host is `mcp.openany.si`. `open_http` takes a URL argument so tests can prove redirects are refused; `exchange` rejects every URL except `https://mcp.openany.si` before that helper runs.
- Python safety lines mention the secret because they pass it into `redact` before building the message. Unit tests use a canary key and require it to be absent from errors and results.
- Financial-activity warnings match the words "charge" and "pay". The plugin does not take a card, move funds, or transfer tokens. OpenAny bills `run_api` on the caller's own key. Say that in the marketplace PR. Do not delete the charge warning to silence the scanner.

## What is still manual

1. A person reviews this OpenAny pull request.
2. That person confirms who will own the Dify Marketplace publisher identity. `openanysi` is only the manifest slug. This package does not create or verify a Dify organization, and it does not claim Verified status.
3. Install `openany-0.1.0.difypkg` on current Dify Community and, if the listing will be offered there, Dify Cloud. Exercise overview, search, get, a rejected `run_api` with an empty secret, and one approved `run_api` with a key the operator is willing to spend. This change did not do that runtime install.
4. Fork `langgenius/dify-plugins`, branch from current `main`, and add one file: `<author>/openany/openany-0.1.0.difypkg`. One package per pull request.
5. Fill the live template in English. Do not check the "tested on Community and Cloud" box until step 3 happened. Write the limitation in Reviewer notes instead.
6. The plugin is free to install. Say that users bring an OpenAny key and pay OpenAny per `run_api` call. Do not claim a Dify fee or a revenue share. The marketplace text fetched for this change says the marketplace accepts free plugins.
7. Do not send mail from this repository. `team@openany.si` is the public contact already used by the project.

`minimum_dify_version` is intentionally absent. Set it only after a runtime install shows the oldest Dify version that loads this package.
