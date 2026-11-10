# API

Everything the web app does goes through `/api/v1`. Authenticate with a session cookie, a personal token (`Authorization: Bearer pcsl_…`, Settings → API & shortcuts) or, from another Pocket app, a Pocket Core link token.

Times are ISO 8601 UTC. A capsule's `state` is `filling`, `sealed` or `opened`; `ready` (in lists and views) means sealed and its day has come.

**Sealed means sealed.** While a capsule is sealed, no endpoint returns what is inside — not to its maker, a box admin, search, the day page or a linked app. Views say only how many things of each kind there are and from whom.

## Capsules

| | |
|---|---|
| `GET /capsules?state=ready\|filling\|sealed\|opened` | The capsules you made or are in |
| `POST /capsules` | `{title, template, about, opens_at ("2027-01-01T09:00" or a date), open_rule: date\|birthday\|anniversary, for_name, for_date, for_age, surprise, slot, deliver: on_open\|on_date, remind_days, sections: [weather, reading, listening, places, health, wealth], members: [emails], recipients: [{name, email}], reply_to}` |
| `GET /capsules/ID` | `{capsule, role, ready, opens_in, items (what you may see), hidden, kinds, from, members, recipients, give_url, can_add, can_open, template}` |
| `PUT /capsules/ID` | Change it (maker). Sealed: it may wait longer, never open sooner |
| `DELETE /capsules/ID` | Throw it away (sealed: `?confirm=1`) |
| `POST /capsules/ID/seal` | Seal it; keeps the world of the day; puts the opening day in Pocket Calendar if set |
| `POST /capsules/ID/open` | Open it on (or after) its day — the maker or anyone in it |
| `POST /capsules/ID/break` | `{confirm: "open", reason}` — the maker breaks the seal early (on record) |
| `PUT /capsules/ID/reflection` | `{reflection, then_now}` after opening |
| `POST /capsules/ID/reply` | Start the next capsule from an opened one |
| `GET /capsules/ID/history` | What happened to it |
| `GET /capsules/ID/snapshot` | What sealing would keep today; `GET /capsules/ID/now` the same parts today (opened) |
| `POST /capsules/ID/members` | `{email, role: contributor\|viewer}`; `DELETE /capsules/ID/members/UID` |
| `POST /capsules/ID/recipients` | `{name, email}`; `DELETE …/recipients/RID`; `POST …/recipients/RID/send` emails their link now |
| `PUT /capsules/ID/give` | `{open, new_key}` — the "add something" link; `POST /capsules/ID/ask` `{to: [{name, addr}], text}` emails it |
| `POST /capsules/ID/calendar` | The opening day into Pocket Calendar |

## What is inside

| | |
|---|---|
| `POST /capsules/ID/items` | `{kind: letter\|photo\|audio\|video\|file\|prediction\|question\|goal\|list\|link, title, body, upload_id, confidence, entries: [{label, value}], url}` — through the slot of a sealed one too (the answer is then just `{id, hidden: true}`) |
| `PUT /items/ID` | Your own while filling; after opening `{outcome: yes\|partly\|no, answer}` |
| `DELETE /items/ID` | Your own, while filling |
| `PUT /capsules/ID/order` | `{ids: […]}` |
| `POST /uploads` | multipart `capsule`, `file` (25 MB), optional `thumb`, `width`, `height` → `{upload: {id}}` |
| `GET /uploads/ID` | The file, if you may see the item it belongs to |

## For you and the rest of the box

| | |
|---|---|
| `GET /home` | Counts, the next to open, yearly capsules due |
| `GET /summary` | `{text, ready, upcoming, filling, opened}` — for Homepage, Pocket AI |
| `GET /day/YYYY-MM-DD` | What happened that day (sealed, opened, put in, opens) — for Pocket Day |
| `GET /search?q=` | Capsules by their outside; things inside opened ones |
| `GET /events?days=30` | Recent history across your capsules |
| `GET /predictions` | Every guess and goal from opened capsules, how many came true, by how sure you were |
| `GET /templates`, `GET /sections` | Kinds of capsule (prompts, questions, guesses, lists); parts of the world of the day |
| `GET/POST /rituals`, `PUT/DELETE /rituals/ID` | Yearly capsules `{title, template, month, day, years, active}` |

## Without an account

| | |
|---|---|
| `GET /public/o/KEY` | A recipient's link: the outside and a countdown before it opens, everything after |
| `GET /public/o/KEY/uploads/ID` | A file of an opened capsule |
| `GET /public/give/KEY` | The "add something" page |
| `POST /public/give/KEY` | multipart `name`, `title`, `letter`, `photo` (+ `thumb`, `width`, `height`) — ten a hour per address |

## Other Pocket apps

Through Pocket Core app links (each switched on per person), Capsule reaches: Calendar (`calendar:write`), Contacts (`contacts:read`), Family (`family:read`), Email (`mail:send`), Chat (`chat:read`, `chat:write`), Photos (`photos:read`), Notes (`notes:read`, `notes:write`), Files (`files:write`), Voice (`voice:read`), Tasks (`tasks:write`), Alerts (`alerts:write`), Blog (`blog:write`), and for the world of the day Weather (`weather:read`), Bookshelf (`books:read`), Music (`listening:read`), Movement (`movement:read`), Health (`health:read`), Wealth (`wealth:read`). Routes under `/apps/*` do that work.

Capsule offers `capsule:read` (lists, a capsule's outside, opened ones' insides, summary, day, search, predictions) and `capsule:write` (start a capsule, put things in — never seal or open; a linked app changes only what it put in). Events: `capsule.sealed`, `capsule.ready`, `capsule.opened`, `capsule.item_added`.

## Admin and data

`GET /backup` (admin: the whole database), `GET /export.json`, `POST /import` (the export file as the body; twice = once), `GET /export.csv`, `GET/PUT /settings`, `GET /settings/system`, `GET/POST/DELETE /tokens`, `GET/POST/PUT/DELETE /users`. Health: `GET /healthz`, `GET /readyz`, `GET /metrics` (`METRICS_ENABLED`).
