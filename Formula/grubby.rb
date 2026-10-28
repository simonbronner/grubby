class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "19"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v19/grubby_v19_darwin_arm64.tar.gz"
      sha256 "a261b62a23c762a663dddd71915141fae942217116e17489413782e535f4be09"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v19/grubby_v19_darwin_amd64.tar.gz"
      sha256 "f3712255e2210e545d144800c6a2b211c03612e9ea718b57d566c5291513f9a7"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v19/grubby_v19_linux_arm64.tar.gz"
      sha256 "e2ebc9cd8eb8f6dbbce9d6daf53c2f1dedd1cdb1ea0e1a193a8c83769266e344"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v19/grubby_v19_linux_amd64.tar.gz"
      sha256 "3de311b83bca3c03c02eec408e28be5cf3f795f9191ac2a40539a528f674306c"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
