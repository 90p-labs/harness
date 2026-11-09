# Style

**Name the content, keep the code small, and expose behavior.** These are conventions. The lint rules of the packs do not check them, so a reviewer checks them by reading.

## Code

- A helper that only one class uses is a private method of that class. It is not a file of its own.
- The domain exposes behavior, not the getters and setters of its state. A caller asks the aggregate to do something.
- The name of an error is the name of its type. The message does not repeat it.
- Write no port before the slice that implements and tests it. A port with no caller has no proven need.

## Comments

- The default is no comment. Rename, extract or split first.
- Write a comment for four reasons only: an external constraint, a trap, a measured value, or a rejected obvious approach.
- A comment has one or two lines and sits above the code that it explains.
- Never write a reference to another file. It goes out of date without a warning.
- Never write a narration of the line, a note about the change or a "ported from" note.
- Delete a comment with the code that it describes.

## Names

- Name the content, not the pattern. An umbrella name tells nothing: utils, helpers, misc, common, manager, impl.
- A file name does not repeat the name of a layer or role folder above it, such as domain or commands. The context name and the role suffix are allowed.
- The role suffix of a file is the exception: it is required (`ddd/file-suffix`).
- Use one naming case for the files across the project. Keep the case that the project already uses.
- A name is a term of the domain. Use the words of the glossary of the project.
