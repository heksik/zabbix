// +build !windows

/*
** Zabbix
** Copyright (C) 2001-2019 Zabbix SIA
**
** This program is free software; you can redistribute it and/or modify
** it under the terms of the GNU General Public License as published by
** the Free Software Foundation; either version 2 of the License, or
** (at your option) any later version.
**
** This program is distributed in the hope that it will be useful,
** but WITHOUT ANY WARRANTY; without even the implied warranty of
** MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
** GNU General Public License for more details.
**
** You should have received a copy of the GNU General Public License
** along with this program; if not, write to the Free Software
** Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.
**/

package vfsfs

import (
	"bufio"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
	"zabbix.com/pkg/plugin"
)

func (p *Plugin) getFsInfoStats() (data []*FsInfoNew, err error) {
	allData, err := p.getFsInfo()
	if err != nil {
		return nil, err
	}

	fsmap := make(map[string]*FsInfoNew)
	for _, info := range allData {
		bytes, err := getFsStats(*info.FsName)
		if err != nil {
			p.Debugf(`cannot discern stats for the mount: %s`, *info.FsName)
			continue
		}

		inodes, err := getFsInode(*info.FsName)
		if err != nil {
			p.Debugf(`cannot discern inode for the mount: %s`, *info.FsName)
			continue
		}

		if bytes.Total > 0 && inodes.Total > 0 {
			fsmap[*info.FsName] = &FsInfoNew{info.FsName, info.FsType, nil, bytes, inodes}
		}
	}

	allData, err = p.getFsInfo()
	if err != nil {
		return nil, err
	}

	for _, info := range allData {
		if fsInfo, ok := fsmap[*info.FsName]; ok {
			data = append(data, fsInfo)
		}
	}

	return
}

func (p *Plugin) readMounts(file io.Reader) (data []*FsInfo, err error) {
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		mnt := strings.Split(line, " ")
		if len(mnt) < 3 {
			p.Debugf(`cannot discern the mount in given line: %s`, line)
			continue
		}
		data = append(data, &FsInfo{FsName: &mnt[1], FsType: &mnt[2]})
	}

	if err = scanner.Err(); err != nil {
		return nil, err
	}

	return
}

func (p *Plugin) getFsInfo() (data []*FsInfo, err error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err = p.readMounts(file)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func getFsStats(path string) (stats *FsStats, err error) {
	fs := unix.Statfs_t{}
	err = unix.Statfs(path, &fs)
	if err != nil {
		return nil, err
	}

	total := fs.Blocks * uint64(fs.Bsize)
	free := fs.Bavail * uint64(fs.Bsize)
	used := total - free
	stats = &FsStats{
		Total: total,
		Free:  free,
		Used:  used,
		PFree: percent(100*fs.Bavail) / percent(fs.Blocks-fs.Bfree+fs.Bavail),
		PUsed: 100 - percent(100*fs.Bavail)/percent(fs.Blocks-fs.Bfree+fs.Bavail),
	}

	return
}

func getFsInode(path string) (stats *FsStats, err error) {
	fs := unix.Statfs_t{}
	err = unix.Statfs(path, &fs)
	if err != nil {
		return nil, err
	}

	total := fs.Files
	free := fs.Ffree
	used := fs.Files - fs.Ffree
	stats = &FsStats{
		Total: total,
		Free:  free,
		Used:  used,
		PFree: 100 * percent(free) / percent(total),
		PUsed: 100 * percent(total-free) / percent(total),
	}

	return
}

func init() {
	plugin.RegisterMetrics(&impl, "VfsFs",
		"vfs.fs.discovery", "List of mounted filesystems. Used for low-level discovery.",
		"vfs.fs.get", "List of mounted filesystems with statistics.",
		"vfs.fs.size", "Disk space in bytes or in percentage from total.",
		"vfs.fs.inode", "Disk space in bytes or in percentage from total.",
	)
}
