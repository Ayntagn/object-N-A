// Package templates supplies reusable Markdown for new and existing notes.
package templates

import (
	"strings"
	"time"
)

type Builtin struct {
	Label string
	body  string
}

func (b Builtin) Markdown(now time.Time) string {
	return strings.ReplaceAll(b.body, "{date}", now.Format("January 2, 2006"))
}

// General and Vault follow the menu order observed in EV-UI-116.
// The sample wording is maintained with the replica.
var General = [...]Builtin{
	{"README", `# Project Name

Describe the project and what it is for.

## Features

- First feature
- Second feature
- Third feature

## Installation

` + "```bash\n# Add the installation steps\n```" + `

## Usage

` + "```bash\n# Add a usage example\n```" + `

## Contributing

Explain how to report an issue or propose a change.

## License

MIT License`},
	{"Meeting Notes", `# Meeting Notes

**Date:** {date}
**Attendees:**

## Agenda

1. First topic
2. Second topic
3. Third topic

## Discussion

### First Topic

Record the main points.

### Second Topic

Record decisions and questions.

## Action Items

- [ ] First follow-up - @owner
- [ ] Second follow-up - @owner
- [ ] Third follow-up - @owner

## Next Meeting

**Date:** TBD
**Topics:**`},
	{"Blog Post", `# Blog Post Title

*Published: {date}*

**TL;DR:** Summarize the post in a sentence.

---

## Introduction

Introduce the topic and give readers a reason to continue.

## Main Content

### First Section

Develop the first point.

### Second Section

Add an example or a second point.

## Conclusion

Bring the main points together.

---

*Thanks for reading.*`},
	{"Project Plan", `# Project Plan: [Project Name]

## Overview

**Project Goal:**
**Start Date:** {date}
**Target Completion:**
**Project Lead:**

## Objectives

1. First objective
2. Second objective
3. Third objective

## Milestones

| Milestone | Description | Due Date | Status |
| --- | --- | --- | --- |
| M1 | Define the scope | Date | 🔴 Not Started |
| M2 | Deliver a first version | Date | 🔴 Not Started |
| M3 | Review the release | Date | 🔴 Not Started |

## Tasks

### Phase 1: Planning

- [ ] Agree on scope
- [ ] Set milestones

### Phase 2: Development

- [ ] Build the first version
- [ ] Review the implementation

### Phase 3: Testing

- [ ] Run acceptance checks
- [ ] Record outstanding issues

## Resources

- Project brief
- Team contacts

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Schedule delay | High | Review the schedule after each milestone |`},
	{"Changelog", `# Changelog

Record the changes users will notice in each release.

Use [Keep a Changelog](https://keepachangelog.com/) to organize entries.
Number releases with [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Describe a new feature.

### Changed
- Describe a change in behavior.

### Fixed
- Describe a resolved issue.

## [1.0.0] - {date}

### Added
- First release.
- List the features shipped in this release.

---

[Unreleased]: https://github.com/owner/project/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/owner/project/releases/tag/v1.0.0`},
}

var Vault = [...]Builtin{
	{"Vault Note", `---
tags: [topic, status/draft]
created: {date}
---

# Note Title

## Summary

Write a short summary.

## Details

Develop the idea here. Link to [[Related Note]] or [[Related Note#Section]] for background.

## Links

- Related: [[]]
- Source: [[]]
- See also: [[]]

> [!note] Context
> Add background that helps explain the note.

## Tags

#topic #research`},
	{"Vault Meeting", `---
tags: [meeting, status/active]
date: {date}
---

# Meeting Notes {date}

**Date:** {date}
**Attendees:** [[Person 1]], [[Person 2]]
**Project:** [[Project Name]]

## Agenda

1. Review [[Previous Meeting Notes]]
2. Discuss the first topic.
3. Discuss the second topic.

## Discussion

### First Topic

Capture the main points.

> [!info] Decision
> Record what the group agreed to.

### Second Topic

Capture open questions.

## Action Items

- [ ] Assign the first action to [[Person 1]] #action
- [ ] Assign the second action to [[Person 2]] #action
- [ ] Follow up on [[Related Topic]] #followup

## Next Steps

- Next meeting: TBD
- Review: [[Project Name#Milestones]]

> [!warning] Blockers
> Note what could delay the next step.

#meeting #team`},
	{"Vault Daily", `---
tags: [daily, journal]
date: {date}
---

# {date}

## Focus

Choose a priority for the day.

- [ ] First priority
- [ ] Second priority
- [ ] Third priority

## Notes

Capture observations and ideas as they come up.

## Links Created

- [[]]

## End of Day

> [!summary] Recap
> Write what you finished and what needs another day.

### Completed
- [x]

### Carry Over
- [ ]

#daily #journal`},
}
