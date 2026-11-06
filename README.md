# benchmark-report (to extract)

A generic renderer for Google Benchmark JSON output plus host hardware
info, as an HTML fragment -- with thin wrappers to emit that same content
as a standalone HTML document or a Doxygen `@page`.

This directory is meant to become its own repository, consumed by other
projects via `FetchContent` at a pinned tag, the same way this project
already consumes Catch2/Blaze/Google Benchmark itself. Until then it lives
here, kept deliberately free of any dependency on kora, CMake, or Doxygen
specifics, so extracting it later is just moving this directory into a new
repo -- not a rewrite.
