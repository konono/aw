# aw demo — VHS (https://github.com/charmbracelet/vhs)
# Usage: vhs demo.tape

Output demo.gif
Set Width 1200
Set Height 700
Set FontSize 16
Set FontFamily "Menlo"
Set Theme "Dracula"
Set Padding 20
Set TypingSpeed 80ms

# --- Scene 1: Install ---
Type "# Install"
Enter
Sleep 500ms
Type "go install github.com/konono/aw@latest"
Enter
Sleep 2s

# --- Scene 2: Show profiles ---
Type "# Available profiles"
Enter
Sleep 500ms
Type "aw profiles"
Enter
Sleep 3s

# --- Scene 3: Launch ---
Type "# Launch Claude Code in a disposable container"
Enter
Sleep 500ms
Type "aw"
Enter
Sleep 6s

# --- Scene 4: Interact ---
Type "hi"
Enter
Sleep 4s
