package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/crypto/bcrypt"
)

const (
	clientAppName           = "client"
	serverAppName           = "server"
	directConnectionTimeout = 5 * time.Second
	defaultRelayControlAddr = "193.23.218.76:34000"
	effectiveHostIDPrefix   = "EFFECTIVE_HOST_ID:"
	bcryptCost              = 12
)

func getExecutablePath(appName string) (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get current executable path: %w", err)
	}
	dir := filepath.Dir(exePath)
	baseName := appName
	if runtime.GOOS == "windows" {
		baseName += ".exe"
	}
	return filepath.Join(dir, baseName), nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	fyneApp := app.New()
	mainWindow := fyneApp.NewWindow("Application Launcher")
	mainWindow.SetFixedSize(true)

	relayServerEntry := widget.NewEntry()
	relayServerEntry.SetPlaceHolder("Relay Server IP:Port (e.g., " + defaultRelayControlAddr + ")")
	relayServerEntry.SetText(defaultRelayControlAddr)

	hostButton := widget.NewButton("Become a Host (Direct & Relay)", func() {
		log.Println("INFO: 'Become a Host' clicked.")

		passwordEntryWidget := widget.NewPasswordEntry()
		passwordEntryWidget.SetPlaceHolder("Leave empty for no password")

		allowMouseControlCheck := widget.NewCheck("Allow Mouse Control", nil)
		allowMouseControlCheck.SetChecked(true)
		allowKeyboardControlCheck := widget.NewCheck("Allow Keyboard Control", nil)
		allowKeyboardControlCheck.SetChecked(true)
		allowFileSystemAccessCheck := widget.NewCheck("Allow File System Access", nil)
		allowFileSystemAccessCheck.SetChecked(true)
		allowTerminalAccessCheck := widget.NewCheck("Allow Terminal Access", nil)
		allowTerminalAccessCheck.SetChecked(true)

		serverRelaxedAuthCheck := widget.NewCheck("Enable Relaxed Local Authentication (for server)", nil)
		serverRelaxedAuthCheck.SetChecked(false)
		serverHeadlessCheck := widget.NewCheck("Run Server Headless (No GUI)", nil)
		serverHeadlessCheck.SetChecked(false)

		formItems := []*widget.FormItem{
			{Text: "Session Password", Widget: passwordEntryWidget, HintText: "Enter a password for this session."},
			{Text: "Mouse Control", Widget: allowMouseControlCheck},
			{Text: "Keyboard Control", Widget: allowKeyboardControlCheck},
			{Text: "File System Access", Widget: allowFileSystemAccessCheck},
			{Text: "Terminal Access", Widget: allowTerminalAccessCheck},
			{Text: "Server Mode", Widget: serverHeadlessCheck, HintText: "Run server without a graphical interface."},
			{Text: "Advanced", Widget: serverRelaxedAuthCheck, HintText: "Allows clients on local network to connect more easily if they skip server certificate validation."},
		}

		passwordDialog := dialog.NewForm("Set Host Options", "Set", "Cancel", formItems, func(ok bool) {
			if !ok {
				log.Println("INFO: Host cancelled options input.")
				return
			}

			plainPassword := passwordEntryWidget.Text
			hashedPassword := ""

			allowMouse := allowMouseControlCheck.Checked
			allowKeyboard := allowKeyboardControlCheck.Checked
			allowFS := allowFileSystemAccessCheck.Checked
			allowTerminal := allowTerminalAccessCheck.Checked
			enableServerRelaxedAuth := serverRelaxedAuthCheck.Checked
			enableHeadless := serverHeadlessCheck.Checked

			if plainPassword == "" {
				log.Println("INFO: Host chose not to set a password.")
			} else {
				log.Println("INFO: Host set a password. Hashing it...")
				hashBytes, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcryptCost)
				if err != nil {
					log.Printf("ERROR: Failed to hash password: %v", err)
					dialog.ShowError(fmt.Errorf("Failed to secure password: %v", err), mainWindow)
					return
				}
				hashedPassword = string(hashBytes)
				log.Println("INFO: Password hashed successfully.")
			}
			log.Printf("INFO: Server will launch with Headless: %t, Relaxed Local Auth: %t, Mouse: %t, Keyboard: %t, FS: %t, Terminal: %t",
				enableHeadless, enableServerRelaxedAuth, allowMouse, allowKeyboard, allowFS, allowTerminal)
			launchServerProcess(mainWindow, fyneApp, relayServerEntry.Text, hashedPassword, enableServerRelaxedAuth,
				allowMouse, allowKeyboard, allowFS, allowTerminal, enableHeadless)
		}, mainWindow)
		passwordDialog.Resize(fyne.NewSize(950, 330))
		passwordDialog.Show()
	})

	connectButton := widget.NewButton("Connect to Remote PC", func() {
		log.Println("INFO: 'Connect to Remote PC' clicked.")
		currentRelayAddr := relayServerEntry.Text
		if currentRelayAddr == "" {
			currentRelayAddr = defaultRelayControlAddr
		}
		promptForAddressAndPasswordAndConnect(mainWindow, fyneApp, currentRelayAddr)
	})

	mainWindow.SetContent(container.NewVBox(
		widget.NewLabel("Choose your role:"),
		container.NewBorder(nil, nil, widget.NewLabel("Relay Server:"), nil, relayServerEntry),
		hostButton,
		connectButton,
	))
	mainWindow.Resize(fyne.NewSize(750, 430))
	mainWindow.SetFixedSize(false)
	mainWindow.CenterOnScreen()
	mainWindow.ShowAndRun()
}

func launchServerProcess(parentWindow fyne.Window, fyneApp fyne.App, relayAddr, hashedPassword string, enableRelaxedAuth bool,
	allowMouse, allowKeyboard, allowFS, allowTerminal bool, enableHeadless bool) {
	serverPath, err := getExecutablePath(serverAppName)
	if err != nil {
		log.Printf("ERROR: Could not determine path for server: %v", err)
		dialog.ShowError(fmt.Errorf("Could not find server application '%s': %v", serverAppName, err), parentWindow)
		return
	}
	currentRelayAddr := relayAddr
	if currentRelayAddr == "" {
		currentRelayAddr = defaultRelayControlAddr
	}

	args := []string{"-relay=true", "-hostID=LauncherHost", "-relayServer=" + currentRelayAddr}
	if hashedPassword != "" {
		args = append(args, "-sessionPassword="+hashedPassword)
	}
	if enableRelaxedAuth {
		args = append(args, "-localRelaxedAuth=true")
	}
	if enableHeadless {
		args = append(args, "-headless=true")
	}
	args = append(args, fmt.Sprintf("-allowMouseControl=%t", allowMouse))
	args = append(args, fmt.Sprintf("-allowKeyboardControl=%t", allowKeyboard))
	args = append(args, fmt.Sprintf("-allowFileSystemAccess=%t", allowFS))
	args = append(args, fmt.Sprintf("-allowTerminalAccess=%t", allowTerminal))

	cmd := exec.Command(serverPath, args...)
	log.Printf("INFO: Launching server with args: %v", args)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("ERROR: Failed to create stdout pipe for server: %v", err)
		dialog.ShowError(fmt.Errorf("Failed to create stdout pipe: %v", err), parentWindow)
		return
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("ERROR: Failed to create stderr pipe for server: %v", err)
		dialog.ShowError(fmt.Errorf("Failed to create stderr pipe: %v", err), parentWindow)
		return
	}

	err = cmd.Start()
	if err != nil {
		log.Printf("ERROR: Failed to launch server '%s': %v", serverPath, err)
		dialog.ShowError(fmt.Errorf("Failed to launch server: %v", err), parentWindow)
		return
	}
	log.Printf("INFO: Server '%s' launched (PID: %d). Headless: %t, Relay: %s, Password protection: %t, Relaxed Auth: %t, Mouse: %t, Keyboard: %t, FS: %t, Terminal: %t. Waiting for Host ID...",
		serverPath, cmd.Process.Pid, enableHeadless, currentRelayAddr, hashedPassword != "", enableRelaxedAuth, allowMouse, allowKeyboard, allowFS, allowTerminal)

	initialDialogMessage := fmt.Sprintf("Server '%s' launched.\nHeadless: %t\nRelay: %s\nPassword Protected: %t\nRelaxed Local Auth: %t\nMouse: %t, Keyboard: %t, FS: %t, Terminal: %t\nWaiting for Host ID...",
		serverAppName, enableHeadless, currentRelayAddr, hashedPassword != "", enableRelaxedAuth, allowMouse, allowKeyboard, allowFS, allowTerminal)
	initialDialog := dialog.NewInformation("Host Mode", initialDialogMessage, parentWindow)

	// If headless, we might not want to show a blocking dialog, or a less intrusive one.
	// For now, we'll show it, but it could be changed to a notification.
	initialDialog.Show()

	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("SERVER_STDOUT: %s", line)
			if strings.HasPrefix(line, effectiveHostIDPrefix) {
				hostID := strings.TrimSpace(strings.TrimPrefix(line, effectiveHostIDPrefix))
				log.Printf("INFO: Captured Effective Host ID from server: %s", hostID)

				fyneApp.SendNotification(&fyne.Notification{
					Title: "Host ID Ready", Content: fmt.Sprintf("Host ID: %s", hostID),
				})
				initialDialog.Hide()

				idLabel := widget.NewLabel(fmt.Sprintf("Your Host ID: %s", hostID))
				idLabel.Wrapping = fyne.TextWrapWord
				passwordMsg := "Not password protected."
				if hashedPassword != "" {
					passwordMsg = "Session is password protected."
				}
				passwordLabel := widget.NewLabel(passwordMsg)
				headlessMsg := fmt.Sprintf("Server Headless: %t", enableHeadless)
				headlessLabel := widget.NewLabel(headlessMsg)
				relaxedAuthMsg := fmt.Sprintf("Relaxed Local Auth: %t", enableRelaxedAuth)
				relaxedAuthLabel := widget.NewLabel(relaxedAuthMsg)
				permissionsMsg := fmt.Sprintf("Permissions: Mouse: %t, Keyboard: %t, FS: %t, Terminal: %t",
					allowMouse, allowKeyboard, allowFS, allowTerminal)
				permissionsLabel := widget.NewLabel(permissionsMsg)

				copyButton := widget.NewButton("Copy ID", func() {
					parentWindow.Clipboard().SetContent(hostID)
					dialog.ShowInformation("Copied", "Host ID copied to clipboard!", parentWindow)
				})
				vboxItems := []fyne.CanvasObject{
					widget.NewLabel("Server is running and registered."),
					idLabel,
					passwordLabel,
					headlessLabel,
					relaxedAuthLabel,
					permissionsLabel,
					copyButton,
				}
				content := container.NewVBox(vboxItems...)

				// Resize the dialog content slightly to accommodate the new headless label
				// content.Resize(fyne.NewSize(600, 630)) // This might not be necessary or might need adjustment
				dialog.ShowCustom("Host Ready", "Close", content, parentWindow)
				return
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("ERROR: Reading server stdout: %v", err)
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			log.Printf("SERVER_STDERR: %s", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Printf("ERROR: Reading server stderr: %v", err)
		}
	}()

	go func() {
		errWait := cmd.Wait()
		log.Printf("INFO: Server process (PID: %d) exited. Error (if any): %v", cmd.Process.Pid, errWait)
		fyneApp.SendNotification(&fyne.Notification{Title: "Server Stopped", Content: "The host server process has exited."})
	}()
}

func launchClientApplication(clientPath, targetAddress string, isRelayConn bool, sessionToken string, allowLocalInsecure bool, parentWindow fyne.Window) {
	connectionType := "direct"
	if isRelayConn {
		connectionType = "relay"
	}
	log.Printf("INFO: Attempting to launch client for %s (via %s connection). AllowLocalInsecure: %t", targetAddress, connectionType, allowLocalInsecure)

	args := []string{fmt.Sprintf("-address=%s", targetAddress)}
	if isRelayConn {
		args = append(args, "-connectionType=relay")
		args = append(args, fmt.Sprintf("-sessionToken=%s", sessionToken))
	}
	if allowLocalInsecure {
		args = append(args, "-allowLocalInsecure=true")
	}

	cmd := exec.Command(clientPath, args...)
	log.Printf("INFO: Launching client with args: %v", args)

	clientStdout, _ := cmd.StdoutPipe()
	clientStderr, _ := cmd.StderrPipe()

	err := cmd.Start()
	if err != nil {
		errMsg := fmt.Sprintf("Failed to launch client '%s': %v", clientPath, err)
		log.Printf("ERROR: %s", errMsg)
		dialog.ShowError(fmt.Errorf(errMsg), parentWindow)
		return
	}

	go func() {
		scanner := bufio.NewScanner(clientStdout)
		for scanner.Scan() {
			log.Printf("CLIENT_STDOUT: %s", scanner.Text())
		}
	}()
	go func() {
		scanner := bufio.NewScanner(clientStderr)
		for scanner.Scan() {
			log.Printf("CLIENT_STDERR: %s", scanner.Text())
		}
	}()

	successMsg := fmt.Sprintf("Client '%s' launched (PID: %d) targeting %s (via %s).\nAllowLocalInsecure: %t",
		filepath.Base(clientPath), cmd.Process.Pid, targetAddress, connectionType, allowLocalInsecure)
	log.Printf("INFO: %s", successMsg)
	dialog.ShowInformation("Client Mode", successMsg, parentWindow)

	go func() {
		errWait := cmd.Wait()
		log.Printf("INFO: Client process (PID: %d) exited. Error (if any): %v", cmd.Process.Pid, errWait)
	}()
}

func connectViaRelay(targetHostID, plainTextPassword, relayControlAddr string) (connected bool, relayDataAddrForClient string, sessionToken string, err error) {
	log.Printf("INFO: [Relay] Attempting to connect to HostID '%s' via relay server %s (password provided for verification: %t)",
		targetHostID, relayControlAddr, plainTextPassword != "")

	conn, err := net.DialTimeout("tcp", relayControlAddr, 10*time.Second)
	if err != nil {
		return false, "", "", fmt.Errorf("failed to connect to relay control server %s: %w", relayControlAddr, err)
	}
	defer conn.Close()
	log.Printf("INFO: [Relay] Connected to relay control port %s", relayControlAddr)

	var cmdStr string
	if plainTextPassword == "" {
		cmdStr = fmt.Sprintf("INITIATE_CLIENT_SESSION %s\n", targetHostID)
	} else {
		cmdStr = fmt.Sprintf("INITIATE_CLIENT_SESSION %s %s\n", targetHostID, plainTextPassword)
	}

	_, err = fmt.Fprint(conn, cmdStr)
	if err != nil {
		return false, "", "", fmt.Errorf("failed to send INITIATE_CLIENT_SESSION to relay: %w", err)
	}
	log.Printf("INFO: [Relay] Sent to relay: %s", strings.TrimSpace(cmdStr))

	conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, "", "", fmt.Errorf("failed to read response from relay server: %w", err)
	}
	conn.SetReadDeadline(time.Time{})

	response = strings.TrimSpace(response)
	log.Printf("INFO: [Relay] Received from relay: %s", response)
	parts := strings.Fields(response)

	if len(parts) > 0 {
		switch parts[0] {
		case "SESSION_READY":
			if len(parts) < 3 {
				return false, "", "", fmt.Errorf("invalid SESSION_READY response from relay: %s", response)
			}
			dynamicPortStr := parts[1]
			sessionTokenOut := parts[2]
			relayHost, _, err := net.SplitHostPort(relayControlAddr)
			if err != nil {
				return false, "", "", fmt.Errorf("could not parse host from relayControlAddr '%s': %w", relayControlAddr, err)
			}
			finalRelayDataAddr := net.JoinHostPort(relayHost, dynamicPortStr)
			log.Printf("INFO: [Relay] Constructed data address for client: %s", finalRelayDataAddr)
			return true, finalRelayDataAddr, sessionTokenOut, nil
		case "ERROR_HOST_NOT_FOUND":
			return false, "", "", fmt.Errorf("relay server reported HostID '%s' not found", targetHostID)
		case "ERROR_AUTHENTICATION_FAILED":
			return false, "", "", fmt.Errorf("authentication failed for HostID '%s'", targetHostID)
		default:
			return false, "", "", fmt.Errorf("unexpected response from relay: %s", response)
		}
	}
	return false, "", "", fmt.Errorf("empty or invalid response from relay: %s", response)
}

func promptForAddressAndPasswordAndConnect(parentWindow fyne.Window, a fyne.App, relayServerControlAddr string) {
	inputWindow := a.NewWindow("Connect to Host")
	inputWindow.SetFixedSize(true)

	hostIDEntry := widget.NewEntry()
	hostIDEntry.SetPlaceHolder("Host's IP:PORT (direct) or HostID (relay)")

	passwordEntryWidget := widget.NewPasswordEntry()
	passwordEntryWidget.SetPlaceHolder("Password (if host requires it for relay)")

	clientAllowInsecureCheck := widget.NewCheck("Allow Insecure Local Connection (client-side)", nil)
	clientAllowInsecureCheck.SetChecked(false)

	formItems := []*widget.FormItem{
		{Text: "Target Address/HostID", Widget: hostIDEntry},
		{Text: "Password (for Relay)", Widget: passwordEntryWidget},
		{Text: "Advanced (Direct Only)", Widget: clientAllowInsecureCheck},
	}

	form := &widget.Form{
		Items: formItems,
		OnSubmit: func() {
			userInput := hostIDEntry.Text
			plainTextPasswordAttempt := passwordEntryWidget.Text
			enableClientAllowInsecure := clientAllowInsecureCheck.Checked

			if userInput == "" {
				dialog.ShowInformation("Input Required", "Please enter the target address or HostID.", inputWindow)
				return
			}
			inputWindow.Close()

			clientPath, err := getExecutablePath(clientAppName)
			if err != nil {
				log.Printf("ERROR: Could not find client application: %v", err)
				dialog.ShowError(fmt.Errorf("Could not find client application '%s': %v", clientAppName, err), parentWindow)
				return
			}

			isPotentiallyDirect := strings.Contains(userInput, ":") && !strings.ContainsAny(userInput, " \t\n")
			var directErr error = fmt.Errorf("not attempted or not applicable")

			if isPotentiallyDirect {
				log.Printf("INFO: Attempting direct connection to %s (AllowInsecure: %t)...", userInput, enableClientAllowInsecure)

				launchClientApplication(clientPath, userInput, false, "", enableClientAllowInsecure, parentWindow)
				return
			} else {
				log.Printf("INFO: Input '%s' does not look like IP:PORT, proceeding to relay.", userInput)
				directErr = fmt.Errorf("input not in IP:Port format, attempting relay")
			}

			targetHostID := userInput
			log.Printf("INFO: Attempting relay for HostID '%s' using relay %s...", targetHostID, relayServerControlAddr)

			relayConnected, relayedAddressForClient, sessionToken, errRelay := connectViaRelay(targetHostID, plainTextPasswordAttempt, relayServerControlAddr)

			if relayConnected {
				log.Printf("INFO: Connection via relay for HostID '%s' successful. Client to connect to %s.", targetHostID, relayedAddressForClient)

				launchClientApplication(clientPath, relayedAddressForClient, true, sessionToken, enableClientAllowInsecure, parentWindow)
				return
			}

			log.Printf("WARN: Relay connection attempt for HostID '%s' failed: %v", targetHostID, errRelay)
			finalErrMsg := fmt.Sprintf("Failed to connect to target '%s'.\n\nDirect connection note: %v.\nRelay connection attempt: %v.",
				userInput, directErr, errRelay)
			dialog.ShowError(fmt.Errorf(finalErrMsg), parentWindow)
		},
		OnCancel: func() {
			inputWindow.Close()
		},
	}
	inputWindow.SetContent(form)
	inputWindow.Resize(fyne.NewSize(950, 320))
	inputWindow.CenterOnScreen()
	inputWindow.Show()
}
