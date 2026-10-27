class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "17"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v17/grubby_v17_darwin_arm64.tar.gz"
      sha256 "8da08eb9b3220acd27a081081050b2518d20a84e683ed1437c8a1a2193611bc6"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v17/grubby_v17_darwin_amd64.tar.gz"
      sha256 "dee2a99a2d1e7838802d5fd30c6c8abfad74aa7ed3867fec8bc33d500041d78c"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v17/grubby_v17_linux_arm64.tar.gz"
      sha256 "32f7a7589bee18c1ba9d9f426a79ce16202cb99758a230466f2b042f4f4e2ea6"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v17/grubby_v17_linux_amd64.tar.gz"
      sha256 "8d67052bb0c9c7dd091d8e07c9202585c037848edcef250559d8db6863473091"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
