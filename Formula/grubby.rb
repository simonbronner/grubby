class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "1"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v1/grubby_v1_darwin_arm64.tar.gz"
      sha256 "5a33cd4cc95f0caa1b84b270f7425b3b79cfad8e4997f7da4b7a971f3d910d34"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v1/grubby_v1_darwin_amd64.tar.gz"
      sha256 "cd710297bfecab689ddb7947fb7d45ddce2143fa271f634177c369f35cc79bf0"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v1/grubby_v1_linux_arm64.tar.gz"
      sha256 "f76aedfbc6783fe2e865c1428a4e9e7ae363458c6e852e16ef897fbd05bf23a3"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v1/grubby_v1_linux_amd64.tar.gz"
      sha256 "b67b0779fce8fa857317488e7f9d66160166831759d775b30f1f1b71ec230376"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
