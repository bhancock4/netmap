# netmap demo recording
# Install vhs: brew install vhs
# Record: vhs demo.tape

Output demo.gif

Set Shell "zsh"
Set FontSize 14
Set Width 1400
Set Height 800
Set Theme "Dracula"
Set TypingSpeed 60ms

# Start scan
Type "netmap example.com -d 1 -t 30s"
Enter

# Wait for scan to populate
Sleep 8s

# Navigate down through nodes
Down
Sleep 1s
Down
Sleep 1s
Down
Sleep 1s

# Collapse a node
Enter
Sleep 1s

# Expand it back
Enter
Sleep 1s

# Deep scan the selected node
Type "d"
Sleep 6s

# Switch to path view
Type "v"
Sleep 3s

# Traverse the path
Right
Sleep 1.5s
Right
Sleep 1.5s
Left
Sleep 1s

# Return to tree view
Type "v"
Sleep 1s

# Switch to topology graph
Type "t"
Sleep 3s

# Return to tree
Type "t"
Sleep 1s

# Show help
Type "?"
Sleep 3s

# Close help
Type "?"
Sleep 1s

# Quit
Type "q"
Sleep 1s
