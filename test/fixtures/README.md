# Tower fixture bootstrap

This directory is reserved for the synthetic and captured fixtures called out in the foundation spec and roadmap.

- `demo\` - mixed-session cockpit replay fixtures used by `tower-demo`
- `approvals\` - captured approval prompt fixtures for managed Claude sessions
- `conflicts\` - same-file, same-branch, and git-mutation overlap scenarios
- `parked\` - parked-session bundle examples
- `summaries\` - end-of-session and end-of-day summary fixtures

The bootstrap only seeds the demo fixture path; the other categories should be populated with real captured data as the managed runtime and parser work lands.
