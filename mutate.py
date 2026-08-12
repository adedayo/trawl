p = "packaging/homebrew/trawl-cli.rb"
t = open(p).read()
assert "trawl_Darwin_arm64." in t, "target string not present"
t = t.replace("trawl_Darwin_arm64.", "trawl_Linux_arm64.", 1)
open(p, "w").write(t)
print("mutated")
