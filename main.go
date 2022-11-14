package main

import (
	"fmt"
	"github.com/bastjan/netstat"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"os"

	"os/exec"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------------------------------------------------

// All the stuff relating to getting the processes with ports open
// Note that it's a bit buggy atm, a lot of 0s in fields where stuff can't be found.


func checkProcesses() tea.Msg {
	// Use the netstat package to get all the processes listening on ports
	conn, connErr := netstat.TCP.Connections()
	if (connErr != nil) {
		// Error if we fail, rather than running extra code
		return errMsg{connErr}
	}

	// Create a temporary map with process IDs mapped to a Process struct (used to merge processes with multiple ports
	// open).
	Pids := make(map[int]process)

	// We know we didn't fail, so we got ports successfully! Time to loop through them and add them to a Process slice
	for _, processWithPort := range conn {
		// Foreach loop that saves the element to the variable port and ignores the index (why don't more languages let
		// you save the index as well? That's so damn helpful!)

		// Take the process path and just get the binary's name by splitting at / and selecting the last item in the
		// array


		// Check if PID is not 0, if so, then ignore (in 'swap process'? idk what this it tbh, but google said so)
		if processWithPort.Pid != 0 {

			// If the process ID already has a port open (if processId in keys of PIDs)
			if _, ok := Pids[processWithPort.Pid]; ok {

				// Then add just the port from netstat to the Process.Ports slice
				tmpAppend := Pids[processWithPort.Pid]
				tmpAppend.ports = append(tmpAppend.ports, processWithPort.RemotePort)


				Pids[processWithPort.Pid] = tmpAppend

			} else {
				// else create a new Process struct with the info from netstat

				processPath := strings.Split(processWithPort.Exe, "/")
				processName := processPath[len(processPath) - 1]

				Pids[processWithPort.Pid] = process{id: processWithPort.Pid, name: processName, ports: []int{processWithPort.RemotePort}, userId: processWithPort.UserID}

			}
		}

	}

	allProcesses := make([]process,0)

	// We now have a map of all processes with ports open, so loop through all of them and add their value to the model
	for _, processWithPort := range Pids {
		// Add value to allProcesses
		allProcesses = append(allProcesses, processWithPort)
	}

	// return final array of processes as a processesMsg
	return processesMsg(allProcesses)


}

// Function to convert process struct to an array of arrays of strings (for use in rendering table)
func convertToRows(processes []process) []table.Row {
	final := make([]table.Row,0)

	for _, proc := range processes {
		individualRow := make(table.Row, 4)
		individualRow[0] = strconv.Itoa(proc.id)

		individualRow[1] = proc.name

		tmpPortStrings := make([]string,0)
		for _, port := range proc.ports {
			tmpPortStrings = append(tmpPortStrings, strconv.Itoa(port))
		}

		individualRow[2] = strings.Join(tmpPortStrings, ", ")

		individualRow[3] = proc.userId

		final = append(final, individualRow)

	}

	return final
}



// Func to create a command that will kill a given process ID
func terminateProcess(id int) tea.Cmd {
	return func() tea.Msg {
		pid := id

		cmd := exec.Command("kill", "-15", strconv.Itoa(pid))

		// Kill the process with that ID
		err := cmd.Run()
		if err != nil {
			return errMsg{err}
		}
		return killMsg{""}
	}
}

// The data returned
type process struct {
	id int
	name string
	ports []int
	userId string
}

// The types for messages returned for use with bubbletea's TUI
type processesMsg []process
type errMsg struct{ err error }
type killMsg struct{output string}
// A connection. Contains a protocol type (typically tcp or udp), connection status, remote address and port,
// and local address and port.
type connection struct {
	protocol string
	status string

	remotePort string
	remoteAddress string

	localPort string
	localAddress string
}

// The settings struct. Contains all the settings for parsing and rendering the table
type settings struct {
	readOnly   bool           // Allow process termination
	showClosed bool           // Allow closed ports to be displayed
	listenOnly bool           // Filter to ports that are listening
	columns    []table.Column // The columns that have been selected for rendering
}

// ---------------------------------------------------------------------------------------------------------------------

// MODEL
// The bubbletea model, where most of the processed information is stored, ready to be rendered.

type model struct {
	table     			table.Model    	// The table that gets rendered
	rowStarts			[]int			// The end of each process's list of open ports
	processes 			[]process      	// A slice of process structs
	err       			error          	// The most recent error

	// Settings are stored in the settings struct. Includes render and parsing settings
	settings settings

	// Used in help menu
	keys       keyMap 			// The keymap used
	help       help.Model		// The help bubble that gets rendered
	inputStyle lipgloss.Style	// The style used when rendering everything

	// TODO: Allow user to create custom styles? This might be better as a separate module/tool
	// (if it doesn't exist yet).

}

// ---------------------------------------------------------------------------------------------------------------------

// MESSAGES
// Definitions for messages that get sent to bubbletea after processing I/O with tea.cmd

// Util function to get the error text from an errMsg element
func (e errMsg) Error() string { return e.err.Error() }

type processesMsg struct{ 			// A struct comprised of process structs and table rows
	processes []process
	rows []table.Row
	ends []int
}
type errMsg struct{ err error }		// An error message.
type terminateMsg struct{} 			// The message returned when terminating a process doesn't error. This then results
									// in another command being issued to get the latest slice of processes, which
									// should have the terminated process removed if it was successful.

// ---------------------------------------------------------------------------------------------------------------------
// All the stuff relating to the bubbletea TUI

// All the keybinding // keymap stuff

// keyMap defines a set of keybindings. To work for help it must satisfy
// key.Map. It could also very easily be a map[string]key.Binding.
type keyMap struct {
	Up    key.Binding
	Down  key.Binding

	Refresh key.Binding
	Terminate key.Binding

	Help  key.Binding
	Quit  key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Refresh, k.Terminate, k.Help, k.Quit}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down,}, // first column
		{ k.Terminate, k.Refresh,},
		{ k.Help, k.Quit,}, // second column
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r", "u"),
		key.WithHelp("r/u", "refresh"),
	),
	Terminate: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "terminate"),
	),
}



// Start by creating a theme to use in rendering (edit later)
var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))


type model struct {
	// We're only storing info in the table, so just use that
	table table.Model
	procs []process
	err error

	// Help info
	keys       keyMap
	help       help.Model
	inputStyle lipgloss.Style

}

// We want to get the processes on first run, so send that on init
// checkProcesses() is the primary function that returns a bubbletea message. It handles all the other functions,
// processes their outputs, then passes those outputs to other functions.
// It takes an input of the render and parsing settings, so that all the parsing and conversion from process structs to
// strings is done inside a goroutine.

func checkProcesses(settingsInfo settings) tea.Cmd {
	return func() tea.Msg {

		out, err := getLsof()


		if err != nil {
			if !(err.Error() == "1") {

				return processesMsg{nil, nil, nil} // No processes, so return empty process slice
			}
			// Error if we fail, rather than running extra code
			return errMsg{err}
		}

		// We have a string that represents the `lsof` output. Parse that
		// into a slice of process structs with the parseLsof() function
		parsed, err := parseLsof(out, settingsInfo)

		if err != nil {
			return errMsg{err}
		}

		formatted, ends, err := formatLsof(parsed,settingsInfo)

		return processesMsg{parsed, formatted, ends}

	}
}



// getLsof() runs the desired command and returns the output as a raw string or an error
func getLsof() (string, error) {
	// Set the command to use and get the output of that command (as well as any error codes we may encounter)
	// Command is `lsof -i -Pn -F cPnpLT`
	cmd := exec.Command("lsof", "-i", "-Pn", "-F", "cPnpLT")
	out, err := cmd.Output()

	// If the error code is 1, then there are no processes with open ports.
	// Assume anything else is an actual error
	if err != nil {
		// There's an error, return an empty string & the error. We'll parse error code 1 (no processes found) later on.
		return "", err
	}

	// We have valid data, so return it! The out variable is a byte array, so convert it to a string first.
	return string(out), err

}

// parseLsof() takes the raw string output of lsof and converts it to a slice of process structs based on the parsing
// criteria given to it in a settings struct
func parseLsof(raw string, options settings) ([]process, error) {
	// Input will be a string. Processes are separated by \np (newline, then 'p' character)
	separated := strings.Split(raw, "\np")

	// Create a new slice of processes
	allProcesses := make([]process,0)


	// For each process
	for processIndex, processString := range separated {
		// Start by splitting process info by \nf (newline, then 'f' character) to get each port open as a separate item
		// in a slice, as well as the PID, command, and user in the 1st item in the array
		connectionSplit := strings.Split(processString, "\nf")

		// there will always be a 1st (index 0) element, which should contain the following lines:
		// 88917 (process ID - no initial character as we split using that character earlier EXCEPT in process
		// with index 0, as no newline before it)
		// cProcessName
		// LUserName

		// So get that element, and split by newlines. Check if index is 0, and if so, handle first line to include the
		// 'p' character

		processInfo := strings.Split(connectionSplit[0], "\n")
		if processIndex == 0 {
			// remove first character from the first line in processInfo if we're on index 0
			processInfo[0] = processInfo[0][1:]
		}
		pid, err := strconv.Atoi(processInfo[0])
		if err != nil {
			return nil, err
		}

		cmd := processInfo[1][1:]
		user := processInfo[2][1:]

		// Now onto handling ports and addresses. Looping through each one to parse it.
		allConnections := make([]connection, 0)

		// Ignore first element in array, as we've already parsed it
		for _, connectionString := range connectionSplit[1:] {
			valid := true // Store if the connection is valid based on the parsing settings

			tmpConnection := connection{}

			connectionInfo := strings.Split(connectionString, "\n")
			// A port string will consist of a file descriptor (unused), a connection type (TCP or UDP), the
			// information on the connection (localAddress:localPort->remoteAddress:remotePort), the connection status
			// (established, listening, closed, or an empty field), and size of the read/send buffers (unused).

			// Loop through each property in the connection info


			for _, connectionProperty := range connectionInfo {
				if len(connectionProperty) > 0 {

				switch string(connectionProperty[0]){
					// Switch-case for each identifier (with an additional nested switch-case for the "T**= options)
					case "P":
						// P: Protocol
						tmpConnection.protocol = connectionProperty[1:]
						break

					case "n":
						// n: Local and remote addresses and ports

						if connectionProperty[1:] == "*:*" {
							// *:* usually indicates some unimportant connection, so we just make that connection invalid
							// This might be wrong! If you want to submit an issue about this, then feel free!
							valid = false

						} else {

							splitLocalAndRemote := strings.Split(connectionProperty[1:], "->")
							// If there is a ->, then there is a clear local and remote connection
							if len(splitLocalAndRemote) > 1 {
								// Should only be 2 elements in that array: local and remote addr:port pairs
								splitLocalAddressAndPort := strings.Split(splitLocalAndRemote[0], ":")
								splitRemoteAddressAndPort := strings.Split(splitLocalAndRemote[1], ":")

								// Set the struct's data to the parsed output
								tmpConnection.localAddress = splitLocalAddressAndPort[0]
								tmpConnection.localPort = splitLocalAddressAndPort[1]
								tmpConnection.remoteAddress = splitRemoteAddressAndPort[0]
								tmpConnection.remotePort = splitRemoteAddressAndPort[1]

							} else {
								// if not, then assume we're looking at a local port and address
								// Note here, that might be an incorrect assumption, please correct me if I'm wrong :)

								splitLocalAddressAndPort := strings.Split(splitLocalAndRemote[0], ":")
								tmpConnection.localAddress = splitLocalAddressAndPort[0]

								// As localPort is a string, we can handle ports like '*' without conversions.
								tmpConnection.localPort = splitLocalAddressAndPort[1]

							}
						}
						break

					case "T":
						if connectionProperty[0:4]  == "TST=" {
							// TST= : Connection status
							tmpConnection.status = connectionProperty[4:]


							// If the port isn't closed OR we have enabled closed ports
							if connectionProperty[4:] == "CLOSED" && options.showClosed {
								valid = false
							}
							if options.listenOnly && connectionProperty[4:] != "LISTEN" {
								valid = false
							}
						}
						break

					}
				}



			}

			// That connection has been parsed! Time to add it to the slice.
			if valid {
				allConnections = append(allConnections, tmpConnection) // Add the connection to the slice
			}
		}

		// All elements of a process have now been parsed, so create a new process struct with that information and
		// append it to the allProcesses slice.

		// If the process still has a valid connection in it.
		if len(allConnections) > 0 {
			// Then add it to the slice
			allProcesses = append(allProcesses, process{
				id: pid,
				name: cmd,
				username: user,
				connections: allConnections,
			})
		}

	}

	// Gone through all processes, so now return the final slice of process structs
	return allProcesses, nil
}

// formatLsof() takes the slice of process structs given and converts to the table rows that get rendered
func formatLsof(processes []process, options settings) ([]table.Row, []int, error) {
	// Loop through each process, and create a row based on the columns we have, then add that to a row slice
	var rows []table.Row
	var rowStarts []int

	for _, proc := range processes {
		rowStarts = append(rowStarts, len(rows))

		for connIndex, conn := range proc.connections {

			row := make(table.Row, len(options.columns))

			// Loop through each column in options.columns and use a switch-case on its title to get the value to set at its index
			for columnIndex, column := range options.columns {

				value := ""

				switch column.Title {

				case "PID":
					if connIndex == 0 {
						value = strconv.Itoa(proc.id)
					}
					break

				case "Name":
					if connIndex == 0 {
						value = proc.name
					}

					break
				case "Owner":
					if connIndex == 0 {
						value = proc.username
					}
					break

				case "Protocol":
					value = conn.protocol
					break

				case "Address":
					// If there is a remote address, use that
					if conn.remoteAddress != "" {
						value = conn.remoteAddress
					} else {
						// If not, then use local address
						value = conn.localAddress
					}
					break

				case "Port":
					if conn.remotePort != "" {
						value = conn.remotePort
					} else {
						value = conn.localPort
					}
					break

				case "Local Address":
					value = conn.localAddress
					break
				case "Local Port":
					value = conn.localPort
					break
				case "Remote Address":
					value = conn.remoteAddress
					break
				case "Remote Port":
					value = conn.remotePort
					break

				case "Status":
					value = strings.ToTitle(conn.status)
					break


				}
				row[columnIndex] = value

			}
			rows = append(rows, row)

		}




	}

	return rows, rowStarts, nil

}

// ---------------------------------------------------------------------------------------------------------------------

// Func to create a command that will terminate a given process ID
func terminateProcess(id int) tea.Cmd {
	return func() tea.Msg {
		pid := id
		cmd := exec.Command("kill", strconv.Itoa(pid))

		// Terminate the process with that ID. Don't care about the output, so just ignore it
		err := cmd.Run()

		if err != nil {
			return errMsg{err}
		}
		return terminateMsg{}
	}
}

// ---------------------------------------------------------------------------------------------------------------------

// All the stuff relating to the bubbletea TUI. This includes the Init, Update, and View functions.

func (m model) Init() tea.Cmd {
	return checkProcesses
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case processesMsg:
		// We have processes, lets update the model to use the new processes
		m.table.SetRows(msg.rows) // Convert the array of process structs to text for use in rendering
		m.rowStarts = msg.ends // The starts of each process's set of rows
		m.processes = msg.processes
		return m, nil

	case terminateMsg:
		// terminate process worked, so rerender processes table
		return m, checkProcesses(m.settings)

	case errMsg:
		m.err = msg.err
		return m, nil

	case tea.WindowSizeMsg:
		// If we set a width on the help menu it can gracefully truncate
		// its view as needed.
		m.help.Width = msg.Width


	case tea.KeyMsg:
		switch msg.String() {

		case key.Matches(msg, keys.Terminate):
			if len(m.processes) > 0 {
				// Get the id of the currently highlighted process and terminate that process
				cursor := m.table.Cursor()
				for i := 0; i < (len(m.rowStarts) - 1); i += 1 {
					// loop through each row start from 0 to end
					if m.rowStarts[i] >= cursor &&  m.rowStarts[i+1] < cursor {
						// then it's process at position i
						return m, terminateProcess(m.processes[i].id)
					}
				}
				// If this somehow breaks, then I think it's always the last one in the array
				return m, terminateProcess(m.processes[len(m.processes)-1].id)
			}
			return m, nil

		case "/":
			m.help.ShowAll = !m.help.ShowAll
		case "q", "ctrl+c":
			return m, tea.Quit
		case "u", "r":
			return m, checkProcesses
		case "t":
			// get current process at the current cursor index
			if len(m.procs) > 0 {
				cursor := m.table.Cursor()
				return m, terminateProcess(m.procs[cursor].id)
			}
			return m, nil

		}

	case errMsg:
		m.err = msg
		return m, nil // Just ignore errors for now

	case processesMsg:
		// We have processes, lets update the model to use the new processes

		m.table.SetRows(convertToRows(msg)) // Convert the array of process structs to text for use in rendering

		m.procs = msg // Table doesn't allow you to get rows >:( pls merge that pull request charm, ty :)
		return m, nil

	case killMsg:
		// kill process worked? Rerender processes
		return m, checkProcesses
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	helpView := m.help.View(m.keys)
	tableString := baseStyle.Render(m.table.View())
	height := 2 - strings.Count(helpView, "\n")

	final := tableString + strings.Repeat("\n", height) + helpView
	if m.err != nil {
		final += "Error: " + m.err.Error()
	}
	return final
}

func main() {
	columns := []table.Column{
		{Title: "PID", Width: 5},
		{Title: "Name", Width: 24},
		{Title: "Ports", Width: 16},
		{Title: "User ID", Width: 6},
	}

	rows := []table.Row{}
	// Set to empty, then let render fill them out

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(7),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	m := model{t, []process{}, nil, keys, help.New(), baseStyle}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

}


// PUT THIS IN MAIN
//if runtime.GOOS == "windows" {
//fmt.Println("Can't Execute this on a windows machine")
//} else {
//execute()
//}
