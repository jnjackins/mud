# MUD client

## Usage
```
Usage: mud prefix:path ...
  -serve
    	Run session server (default true)
```

`mud` runs one or more MUD sessions. Each argument should contain a prefix and a path
to a directory containing a valid configuration file. For example, `mud a:mage`
would start a session with prefix `a` and load the configuration at
`mage/config.yaml`. See example/config.yaml for a basic example configuration.

Note that starting a session will begin reading input, but not display any output
by default. Instead, output is written to a file `out` in the directory containing
the configuration file. The user is expected to arrange terminals desired
(probably using a tiling terminal manager) and print the output where desired
(e.g. using `tail -f mage/out`). Similarly, configured logs such as a chat log
can be displayed in a separate terminal, and so on.

## Building
To do.

## Screenshot
![Screenshot](/example/screenshot.png?raw=true "Screenshot")