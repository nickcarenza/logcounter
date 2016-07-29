Log Counter
===========

Usage example: `tail -f error.log | logc error info debug`
This will display a table showing the counts of occurences in error.log of the patterns "error", "info" and "debug".

```
PATTERN COUNT
error   7    
info    10   
debug   0    
```

If you want to capture stderr as well as stdout: 2>&1
