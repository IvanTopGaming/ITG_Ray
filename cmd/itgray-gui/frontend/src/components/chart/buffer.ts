// SpeedBuffer is a fixed-capacity FIFO of speed samples that drives the
// LiveChart. Capacity == seconds at 1 Hz (300 = 5 minutes).
export interface Sample {
  upBps: number;
  downBps: number;
  t: number;
}

export class SpeedBuffer {
  private data: Sample[] = [];
  constructor(private capacity: number) {}
  push(s: Sample) {
    this.data.push(s);
    if (this.data.length > this.capacity) this.data.shift();
  }
  values(): Sample[] {
    return this.data;
  }
}
