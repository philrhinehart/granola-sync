  I use granola to record/transcribe meetings and generate notes.

  It stores a local cache of files here:
  ~/Library/Application\ Support/Granola/cache-v3.json

  I also use Logseq to take notes.

  My logseq notes are stored here: /Users/phil/Library/Mobile\ Documents/iCloud\~com\~logseq\~logseq/Documents/AngelList/

  I'd like to develop a program that can monitor for new notes in the local granola cache, and then add the notes to my logseq.

  Notes should either be included on the current days journal page directly, or stored in a separate note and referenced in the days journal page.

  Journal file format for example:
  ls /Users/phil/Library/Mobile\ Documents/iCloud\~com\~logseq\~logseq/Documents/AngelList/journals
  2025_08_25.md 2025_08_31.md 2025_09_07.md 2025_09_26.md 2025_10_01.md 2025_10_12.md 2025_10_22.md 2025_10_27.md 2025_11_05.md 2025_11_15.md 2025_11_23.md 2025_12_07.md 2025_12_21.md 2026_01_11.md 2026_01_16.md 2026_01_25.md 2026_08_22.md
  2025_08_26.md 2025_09_03.md 2025_09_14.md 2025_09_27.md 2025_10_05.md 2025_10_13.md 2025_10_23.md 2025_10_28.md 2025_11_09.md 2025_11_16.md 2025_11_25.md 2025_12_08.md 2025_12_28.md 2026_01_13.md 2026_01_21.md 2026_01_27.md 2027_02_20.md
  2025_08_28.md 2025_09_04.md 2025_09_21.md 2025_09_28.md 2025_10_09.md 2025_10_19.md 2025_10_24.md 2025_11_02.md 2025_11_10.md 2025_11_17.md 2025_11_30.md 2025_12_14.md 2026_01_04.md 2026_01_14.md 2026_01_22.md 2026_01_28.md


  my preference would be to use golang for this app, but im open to options.

  Heres a repo that tries to do something similar (monitoring the cache). it might be helpful, but i haven't read the code, used it or vetted it. so dont give it too much weight, but might be useful to get some ideas/context for how to handle this.
  https://github.com/owengretzinger/granola-webhook

  Ideally, we can launch this program to have it run in the background without needing to know/do anything. and it should be able to cleanly come back up after restart.
