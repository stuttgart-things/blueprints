# Test Document

This is a test document with various markdown linting issues.

##Missing space after hash

This header above is missing a space after the hash marks.

## Proper Header

Some content here.

##Another Bad Header
Content without proper spacing.

## List Issues

* Item 1 using asterisk
- Item 2 using dash (inconsistent with above)
+ Item 3 using plus

## Trailing Whitespace

This line above has trailing whitespace.

## Empty Lines



Too many empty lines above (should be max 2).

## Code Blocks

```
code without language specified
def hello():
    print("hello")
```

## Very Long Line That Exceeds The Maximum Line Length Configured In The Markdown Linter Configuration File And Should Trigger A Warning

This is a paragraph with a very long line that goes on and on and on and on and on and on and exceeds the maximum line length of 120 characters configured in mdlrc.

## Missing Blank Lines
Immediately followed by text without a blank line.
This can cause rendering issues.

## Links

[Bad link spacing](https://example.com)should have space after.

## Multiple Headers

## Duplicate Header

Some content.

## Duplicate Header

More content with the same header name.

##Emphasis Issues

**Bold text**should have space after.
*Italic text*should also have space.

## End of document without newline
