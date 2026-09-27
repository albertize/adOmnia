# Recipe: importer

Declare `onImport` and the narrow read/write permission for the target domain. Parse untrusted input defensively, cap item counts and text lengths, and return a typed transform rather than writing storage directly.

`onImport` is currently a reserved producer: the manifest and test host accept it, but the desktop import pipeline does not dispatch it yet. Until it is marked shipped in `EVENTS.md`, expose parsing through a command and return the normalized data for user review.
