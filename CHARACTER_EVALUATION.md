# Character and chronicle review

This is a lightweight review protocol for the first hosted discoveries. It does
not claim the current voices have passed evaluation. Use the same three test
prompts across model changes and compare their saved output.

1. **Routine scene:** a familiar sidewalk after light rain. Check whether it is
   allowed to remain ordinary, and whether each agent avoids manufacturing a
   supernatural event.
2. **Ambiguous evidence:** a distant object partly hidden by fog. Check that the
   Scientist separates what is visible from what they infer, and that the others
   preserve uncertainty instead of treating guesses as facts.
3. **Conflicting memory:** a new photo appears to disagree with a prior chronicle.
   Check that the agents name the earlier claim accurately, reconsider it for a
   concrete reason, and keep minority explanations when agreement is absent.

For each agent's reaction and rebuttal, score each category from 0 (missing) to
2 (clear):

| Category | 0 | 1 | 2 |
| --- | --- | --- | --- |
| Evidence | No visible detail supports the claim | Evidence is named but weakly connected | A specific detail clearly supports the claim |
| Distinct voice | Could be any agent | Some role language, mostly generic | Motivation and phrasing are recognizable without the name |
| Uncertainty | Guess is stated as fact | Caveat appears but conclusion overstates it | Observation, inference, and unknowns remain separate |
| Continuity | Earlier lore is ignored or distorted | Lore is mentioned without effect | A specific earlier belief is preserved, revised, or challenged accurately |
| Rebuttal | Repeats a reaction or ignores others | Names an opposing claim | Answers a named point and explains what changed or stayed uncertain |
| Restraint | Invents danger, ritual, or belief change | Adds interpretation beyond evidence | Lets a minor event remain minor when warranted |
| Historian | Loses dissent or invents consensus | Records some disagreement | Verdict, majority/minority positions, and evidence agree with the debate |

Keep the unlabelled outputs for a blind voice review. Review the saved chronicle
against its reactions and rebuttals; a persuasive style does not make unsupported
historical claims acceptable. Record scores, repeated phrases, fabrication, and
missed dissent. Fix the prompt or validation rule that caused a repeated failure,
then rerun the same cases. Preserve only the final chronicles and the review
notes; ordinary raw debate remains temporary.

For a hosted run, include the photo source, model and prompt version, date, scores,
and anonymized chronicle IDs. Do not add real user photos or account identifiers
to a public evaluation artifact.
