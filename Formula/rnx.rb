class Rnx < Formula
  desc "Cross-platform CLI for Joblet distributed job execution system"
  homepage "https://github.com/ehsaniara/joblet"
  license "MIT"

  # Dynamically fetch latest release info
  def self.get_latest_release
    require 'net/http'
    require 'json'
    require 'uri'

    uri = URI('https://api.github.com/repos/ehsaniara/joblet/releases/latest')

    begin
      response = Net::HTTP.get_response(uri)

      if response.code == '200'
        JSON.parse(response.body)
      else
        # Try to get list of releases as fallback
        fallback_uri = URI('https://api.github.com/repos/ehsaniara/joblet/releases')
        fallback_response = Net::HTTP.get_response(fallback_uri)

        if fallback_response.code == '200'
          releases = JSON.parse(fallback_response.body)
          # Get first non-prerelease or just first release
          stable = releases.find { |r| !r['prerelease'] } || releases.first

          if stable
            opoo "Using release #{stable['tag_name']} (unable to fetch 'latest' tag)"
            stable
          else
            odie "Unable to fetch release information from GitHub. Please check your internet connection and try again."
          end
        else
          odie "Unable to connect to GitHub API (HTTP #{response.code}). Please check your internet connection and try again."
        end
      end
    rescue StandardError => e
      odie "Failed to fetch latest release from GitHub: #{e.message}\n\nPlease check your internet connection or try again later."
    end
  end

  latest_release = get_latest_release
  latest_version = latest_release['tag_name']

  version latest_version.sub(/^v/, '')

  # Dynamic URLs based on latest release
  arch_suffix = Hardware::CPU.intel? ? 'amd64' : 'arm64'
  url "https://github.com/ehsaniara/joblet/releases/download/#{latest_version}/rnx-v#{latest_version.sub(/^v/, '')}-darwin-#{arch_suffix}.tar.gz"

  # Optional Node.js dependency for admin UI
  depends_on "node" => :optional

  # Installation option - admin UI is auto-detected by default
  option "with-admin", "Force installation with web admin UI (requires Node.js)"
  option "cli-only", "Install CLI only (skip admin UI prompt)"

  def install
    # Install base rnx binary (handle different naming conventions)
    if File.exist?("rnx")
      bin.install "rnx"
    elsif File.exist?("rnx-darwin-amd64")
      bin.install "rnx-darwin-amd64" => "rnx"
    elsif File.exist?("rnx-darwin-arm64")
      bin.install "rnx-darwin-arm64" => "rnx"
    else
      # Fallback: find any file starting with rnx
      rnx_files = Dir.glob("rnx*")
      if rnx_files.any?
        bin.install rnx_files.first => "rnx"
      else
        raise "No rnx binary found in the archive"
      end
    end

    # Determine installation type
    ohai "Checking for Node.js installation..."
    install_admin = determine_admin_installation

    if install_admin
      setup_admin_ui
    else
      ohai "✅ RNX CLI installed successfully!"
      ohai "💡 To install the web admin UI later, run:"
      ohai "   brew reinstall rnx --with-admin"
    end

    # Create shell completions (with error handling)
    begin
      output = Utils.safe_popen_read(bin / "rnx", "completion", "bash")
      (bash_completion / "rnx").write output if output && !output.empty?
    rescue => e
      opoo "Could not generate bash completions: #{e.message}"
    end

    begin
      output = Utils.safe_popen_read(bin / "rnx", "completion", "zsh")
      (zsh_completion / "_rnx").write output if output && !output.empty?
    rescue => e
      opoo "Could not generate zsh completions: #{e.message}"
    end

    begin
      output = Utils.safe_popen_read(bin / "rnx", "completion", "fish")
      (fish_completion / "rnx.fish").write output if output && !output.empty?
    rescue => e
      opoo "Could not generate fish completions: #{e.message}"
    end
  end

  def test
    assert_match version.to_s, shell_output("#{bin}/rnx --version")

    # Test admin UI if installed
    admin_dir = share / "rnx/admin"
    if admin_dir.exist?
      assert admin_dir.join("server/package.json").exist?, "Admin server package.json should exist"
      assert admin_dir.join("ui/dist/index.html").exist?, "Admin UI build should exist"
    end
  end

  def caveats
    admin_dir = share / "rnx/admin"

    if admin_dir.exist?
      <<~EOS
        🎉 RNX with Admin UI installed successfully!
        
        📋 Usage:
          CLI commands:     rnx --help
          Web Admin UI:     rnx admin  (or rnx-admin)
        
        🔧 Configuration:
          Copy your rnx-config.yml to: ~/.rnx/
        
        🌐 Admin UI will be available at: http://localhost:5173
        
        💡 Tips:
          - The admin UI provides a visual interface for job management
          - All CLI commands are still available alongside the web interface
          - Admin UI spawns local rnx CLI commands to communicate with joblet servers
      EOS
    else
      <<~EOS
        📱 RNX CLI installed successfully!
        
        📋 Usage: rnx --help
        
        🔧 Configuration:
          Copy your rnx-config.yml to: ~/.rnx/
        
        🌐 To install the web admin UI:
          brew reinstall rnx --with-admin
        
        💡 The web admin UI provides a visual interface for managing jobs,
           viewing logs, and monitoring your joblet servers.
      EOS
    end
  end

  private

  def determine_admin_installation
    # Check for explicit options first
    if build.with? "admin"
      ohai "Installing with admin UI (--with-admin specified)"
      return true
    end

    if build.with? "cli-only"
      ohai "Installing CLI only (--cli-only specified)"
      return false
    end

    ohai "Detecting Node.js for optional admin UI installation..."

    # Try to detect Node.js
    begin
      # Use which_formula to check if node formula is installed
      node_installed = Formula["node"].any_version_installed? rescue false

      # Also check if node command is available in PATH
      node_in_path = !`which node 2>/dev/null`.strip.empty?

      node_available = node_installed || node_in_path

      if node_available
        # Get Node.js version more reliably
        node_path = `which node 2>/dev/null`.strip
        ohai "Node.js path found: #{node_path}" if !node_path.empty?

        node_version = nil

        if !node_path.empty?
          node_version = `#{node_path} --version 2>/dev/null`.strip
          ohai "Node.js version output: '#{node_version}'"
        end

        if node_version && !node_version.empty? && node_version.start_with?("v")
          ohai "✅ Node.js detected: #{node_version}"

          # Verify Node.js version compatibility
          begin
            verify_nodejs_version
          rescue => e
            opoo "Node.js version check warning: #{e.message}"
          end

          # Always prompt if Node.js is available (unless in CI)
          if ENV["CI"].nil?
            ohai "🎨 The admin UI provides a web interface for managing jobs"

            # Use a simple approach that should always work
            print "\n🤔 Would you like to install the web admin UI? [Y/n]: "
            $stdout.flush

            # Try to read input - if it fails, default to no
            begin
              # Give user 10 seconds to respond
              require 'io/console'
              response = nil

              # Simple timeout approach
              thread = Thread.new { response = $stdin.gets }
              thread.join(10) # Wait up to 10 seconds

              if response
                answer = response.strip.downcase
                if answer.empty? || answer == 'y' || answer == 'yes'
                  ohai "Installing with admin UI..."
                  return true
                else
                  ohai "Installing CLI only"
                  return false
                end
              else
                ohai "No response received, defaulting to CLI only"
                ohai "To install with admin UI later, use: brew reinstall rnx --with-admin"
                return false
              end
            rescue => e
              opoo "Could not read input: #{e.message}"
              ohai "Installing CLI only (use --with-admin to force admin UI)"
              return false
            end
          else
            ohai "CI environment detected, installing CLI only"
            return false
          end
        else
          ohai "Node.js found but unable to determine version"
          ohai "Node path: #{node_path}" if !node_path.empty?
          ohai "Version output: #{node_version}" if node_version
          ohai "Installing CLI only (use --with-admin to force admin UI)"
          return false
        end
      else
        ohai "❌ Node.js not detected"

        # Interactive prompt with default No
        if ENV["HOMEBREW_NO_ENV_HINTS"] != "1" && ENV["CI"].nil?
          print "🤔 Would you like to install Node.js and the web admin UI? (y/N): "
          response = STDIN.gets

          if response && response.strip.downcase.start_with?("y")
            # Install Node.js as a dependency
            ohai "Installing Node.js..."
            Formula["node"].install
            # Verify Node.js version after installation
            verify_nodejs_version
            return true
          else
            return false
          end
        else
          ohai "Non-interactive mode, installing CLI only"
          return false
        end
      end
    rescue => e
      # Fallback to CLI-only installation if anything goes wrong
      opoo "Installation detection failed: #{e.message}"
      opoo "Installing CLI only (use --with-admin to force admin UI installation)"
      false
    end
  end

  def setup_admin_ui
    ohai "🔧 Setting up admin UI..."

    # Check if admin files exist in the archive
    if !Dir.exist?("admin")
      opoo "Admin UI files not found in this release"
      opoo "The release may have been built without admin UI support"
      opoo "Please report this issue or try a different release version"
      return false
    end

    # Create admin directory structure (following homebrew conventions)
    admin_dir = share / "rnx/admin"
    admin_dir.mkpath

    # Install admin files from release archive
    ohai "📁 Installing admin UI files..."
    cp_r "admin/.", admin_dir

    # Verify Node.js dependencies are present (pre-installed in release)
    ohai "📦 Verifying admin server dependencies..."
    unless (admin_dir / "server/node_modules").exist?
      onoe "Admin server dependencies not found - installing..."
      cd admin_dir / "server" do
        system "npm", "install", "--production", "--silent"
        unless $?.success?
          onoe "Failed to install admin server dependencies"
          raise "Admin UI setup failed"
        end
      end
    else
      ohai "✅ Admin server dependencies already included"
    end

    # Verify admin UI build exists
    ui_build = admin_dir / "ui/dist/index.html"
    unless ui_build.exist?
      onoe "Admin UI build not found"
      raise "Admin UI setup failed"
    end

    # Create admin launcher script
    create_admin_launcher

    ohai "✅ Admin UI setup complete!"
    ohai "🚀 Usage:"
    ohai "   CLI: rnx --help"
    ohai "   Web UI: rnx admin"

  rescue => e
    onoe "Admin UI setup failed: #{e.message}"
    opoo "Continuing with CLI-only installation..."
  end

  def create_admin_launcher
    # Create rnx-admin wrapper script
    admin_script = bin / "rnx-admin"
    admin_script.write <<~EOS
      #!/bin/bash
      # RNX Admin UI Launcher
      ADMIN_DIR="#{share}/rnx/admin"
      
      if [ ! -d "$ADMIN_DIR" ]; then
        echo "❌ Admin UI not installed. Install with: brew reinstall rnx --with-admin"
        exit 1
      fi
      
      echo "🚀 Starting RNX Admin UI..."
      echo "📍 Admin server directory: $ADMIN_DIR/server"
      echo "🌐 Opening http://localhost:5173 in your browser..."
      
      # Start the admin server
      cd "$ADMIN_DIR/server"
      node server.js &
      
      # Wait a moment for server to start
      sleep 2
      
      # Open browser
      open "http://localhost:5173" 2>/dev/null || echo "💡 Navigate to http://localhost:5173 in your browser"
      
      echo "✅ Admin UI is running!"
      echo "🛑 Press Ctrl+C to stop"
      
      # Keep script running
      wait
    EOS
    admin_script.chmod 0755
  end

  def verify_nodejs_version
    return unless system("which", "node", out: File::NULL, err: File::NULL)

    node_version = `node --version 2>/dev/null`.strip
    return if node_version.empty?

    # Parse version (remove 'v' prefix and get major version)
    version_match = node_version.match(/^v?(\d+)\./)
    return unless version_match

    major_version = version_match[1].to_i

    if major_version < 18
      onoe "Node.js #{node_version} detected, but admin UI requires Node.js 18+"
      ohai "💡 Please upgrade Node.js:"
      ohai "   brew upgrade node"
      ohai "   or visit: https://nodejs.org/"
      raise "Node.js version incompatible"
    end

    ohai "✅ Node.js #{node_version} is compatible"
  end
end
