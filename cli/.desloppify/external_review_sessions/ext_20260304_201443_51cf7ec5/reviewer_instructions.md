# External Blind Review Session

Session id: ext_20260304_201443_51cf7ec5
Session token: f594ceb6e29bb68c47e43dd49b46b340
Blind packet: /home/boboysdadda/projects/gitops_homelab_2_0/cli/.desloppify/review_packet_blind.json
Template output: /home/boboysdadda/projects/gitops_homelab_2_0/cli/.desloppify/external_review_sessions/ext_20260304_201443_51cf7ec5/review_result.template.json
Claude launch prompt: /home/boboysdadda/projects/gitops_homelab_2_0/cli/.desloppify/external_review_sessions/ext_20260304_201443_51cf7ec5/claude_launch_prompt.md
Expected reviewer output: /home/boboysdadda/projects/gitops_homelab_2_0/cli/.desloppify/external_review_sessions/ext_20260304_201443_51cf7ec5/review_result.json

Happy path:
1. Open the Claude launch prompt file and paste it into a context-isolated subagent task.
2. Reviewer writes JSON output to the expected reviewer output path.
3. Submit with the printed --external-submit command.

Reviewer output requirements:
1. Return JSON with top-level keys: session, assessments, findings.
2. session.id must be `ext_20260304_201443_51cf7ec5`.
3. session.token must be `f594ceb6e29bb68c47e43dd49b46b340`.
4. Include findings with required schema fields (dimension/identifier/summary/related_files/evidence/suggestion/confidence).
5. Use the blind packet only (no score targets or prior context).
