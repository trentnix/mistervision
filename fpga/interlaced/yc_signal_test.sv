// SPDX-License-Identifier: GPL-2.0-or-later
// Sample the scaler encoder's digital output before the board's analog DAC.
module yc_signal_test;
    reg clk = 0;
    always #5 clk = !clk;

    reg pal = 1, cvbs = 0, extra_hs = 0, de = 0;
    reg [23:0] rgb = 0;
    reg [39:0] phase_inc = 40'd243740768438;
    reg [16:0] burst_range = {7'd16, 10'd56};
    wire [23:0] video, opposite_line;
    wire de_out;

    interlaced_yc encoder (
        .clk(clk), .phase_inc(phase_inc), .burst_range(burst_range),
        .pal(pal), .cvbs(cvbs), .rgb(rgb), .de(de),
        .hs(1'b0), .vs(1'b0), .cs(1'b0), .video(video),
        .de_out(de_out), .hs_out(), .vs_out(), .cs_out()
    );
    interlaced_yc reference_encoder (
        .clk(clk), .phase_inc(phase_inc), .burst_range(burst_range),
        .pal(pal), .cvbs(cvbs), .rgb(rgb), .de(de),
        .hs(extra_hs), .vs(1'b0), .cs(extra_hs), .video(opposite_line),
        .de_out(), .hs_out(), .vs_out(), .cs_out()
    );

    task check_pal_line_color;
        int changed;
        // An extra horizontal sync puts the reference on the other PAL line
        // parity. Both carriers keep the same phase throughout.
        repeat (200) @(negedge clk);
        extra_hs = 1;
        repeat (60) @(negedge clk);
        extra_hs = 0;
        de = 1;
        // Y = floor((306*40 + 601*0 + 117*252)/1024) = 40. R-Y = 0,
        // so reversing the sign of that component must not alter chroma.
        rgb = 24'h2800fc;
        repeat (150) @(negedge clk);
        repeat (512) begin
            @(negedge clk);
            if (video[23:16] !== opposite_line[23:16])
                $fatal(1, "PAL line parity changed a color with zero R-Y");
        end

        // A nonzero R-Y must respond to the PAL switch. Comparing identical
        // line parities (or disabling color) would also pass the check above.
        rgb = 24'hff0000;
        repeat (24) @(negedge clk);
        changed = 0;
        repeat (512) begin
            @(negedge clk);
            if (video[23:16] != opposite_line[23:16]) changed++;
        end
        if (changed < 256)
            $fatal(1, "PAL line parity failed to reverse red chroma (%0d/512 samples differed)", changed);
    endtask

    task check_picture_edges(input bit use_pal, input bit composite);
        // The black blanking level and full white have known digital codes.
        // In CVBS the mixer includes half the neutral chroma pedestal (128).
        reg [7:0] white, black, expected;
        white = composite ? 8'd191 : 8'd255;
        black = composite ? 8'd64 : 8'd0;
        pal = use_pal;
        cvbs = composite;
        phase_inc = pal ? 40'd243740768438 : 40'd196787592471;
        burst_range = pal ? {7'd16, 10'd56} : {7'd20, 10'd70};
        rgb = 0;
        de = 0;
        repeat (128) @(negedge clk);
        // Test both edges of the same white rectangle. Its brightness must
        // coincide with the delayed active-picture flag at every sample.
        for (int pixel = 0; pixel < 64; pixel++) begin
            de = pixel >= 8 && pixel < 40;
            rgb = de ? 24'hffffff : 24'h000000;
            @(negedge clk);
            expected = de_out ? white : black;
            if (video[15:8] !== expected)
                $fatal(1, "Picture edge: pal=%0d cvbs=%0d pixel=%0d got=%0d expected=%0d",
                       pal, cvbs, pixel, video[15:8], expected);
        end
    endtask

    initial begin
        check_pal_line_color();
        check_picture_edges(0, 0);
        check_picture_edges(0, 1);
        check_picture_edges(1, 0);
        check_picture_edges(1, 1);
        $display("PASS PAL line parity and picture edges for S-Video and CVBS");
        $finish;
    end
endmodule
